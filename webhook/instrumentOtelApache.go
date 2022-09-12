/*
Copyright (c) 2022 Martin Divis.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const OTEL_WEBSERVER_DIR = "/opt/opentelemetry-webserver"
const OTEL_WEBSERVER_AGENT_DIR = OTEL_WEBSERVER_DIR + "/agent"
const OTEL_WEBSERVER_CONFIG_DIR = OTEL_WEBSERVER_DIR + "/source-conf"

func apacheOtelInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}

	containerId := 0

	patchOps = append(patchOps, addOtelApacheEnvVar(pod, instrRule, containerId)...)
	patchOps = append(patchOps, addSpecifiedContainerEnvVars(instrRule.InjectionRules.EnvVars, containerId)...)

	patchOps = append(patchOps, addOtelApacheAgentVolumeMount(pod, instrRule, containerId)...)
	patchOps = append(patchOps, addApacheApplicationContainerCloneAsInit(pod, instrRule, containerId)...)
	patchOps = append(patchOps, dropApachePassedConfig(pod, instrRule, containerId)...)
	patchOps = append(patchOps, addOtelApacheAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addOtelApacheAgentVolume(pod, instrRule)...)
	patchOps = append(patchOps, addOtelApacheSourceConfVolume(pod, instrRule)...)

	if instrRule.InjectionRules.OpenTelemetryCollector != "" {
		otelCollConfig, found := otelCollsConfig[instrRule.InjectionRules.OpenTelemetryCollector]
		if !found {
			log.Printf("Cannot find OTel collector definition %s\n", instrRule.InjectionRules.OpenTelemetryCollector)
		} else {
			if otelCollConfig.Mode == "sidecar" {
				patchOps = append(patchOps, addOtelCollSidecar(pod, instrRule, 0)...)
			}
		}
	} else {
		log.Printf("Cannot find OTel collector definition %v\n", instrRule.InjectionRules)
	}

	return patchOps
}

func addOtelApacheEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	/*
		patchOps = append(patchOps, patchOperation{
			Op:   "add",
			Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
			Value: corev1.EnvVar{
				Name:  "LD_LIBRARY_PATH",
				Value: OTEL_WEBSERVER_AGENT_DIR + "/sdk_lib/lib",
			},
		})
	*/

	return patchOps
}

func addOtelApacheAgentVolumeMount(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	// directory with modified Apache conf directory
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: "/usr/local/apache2/conf",
			Name:      "apache-conf-dir",
		},
	})
	// directory with webserver agent
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: OTEL_WEBSERVER_AGENT_DIR, //TODO
			Name:      "otel-agent-repo-apache", //TODO
		},
	})
	return patchOps
}

func addOtelApacheAgentVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "otel-agent-repo-apache", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addOtelApacheSourceConfVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "apache-conf-dir", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addOtelApacheAgentInitContainer(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/initContainers/-",
		Value: corev1.Container{
			Name:    "otel-agent-attach-apache", //TODO
			Image:   instrRules.InjectionRules.Image,
			Command: []string{"/bin/sh", "-c"},
			Args: []string{
				"cp -ar /opt/opentelemetry/* " + OTEL_WEBSERVER_AGENT_DIR + " && " +
					"export agentLogDir=$(echo \"" + OTEL_WEBSERVER_AGENT_DIR + "/logs\" | sed 's,/,\\\\/,g') && " +
					"cat " + OTEL_WEBSERVER_AGENT_DIR + "/conf/appdynamics_sdk_log4cxx.xml.template | sed 's/__agent_log_dir__/'${agentLogDir}'/g'  > " + OTEL_WEBSERVER_AGENT_DIR + "/conf/appdynamics_sdk_log4cxx.xml &&" +
					"echo \"$OPENTELEMETRY_MODULE_CONF\" > " + OTEL_WEBSERVER_CONFIG_DIR + "/opentelemetry_module.conf && " +
					"cat " + OTEL_WEBSERVER_CONFIG_DIR + "/opentelemetry_module.conf && " +
					"echo 'Include /usr/local/apache2/conf/opentelemetry_module.conf' >> " + OTEL_WEBSERVER_CONFIG_DIR + "/httpd.conf",
			},
			ImagePullPolicy: corev1.PullAlways,
			Env: []corev1.EnvVar{
				{
					Name:  "OPENTELEMETRY_MODULE_CONF",
					Value: getApacheOtelConfig(pod, instrRules),
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    limCPU,
					corev1.ResourceMemory: limMem,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    reqCPU,
					corev1.ResourceMemory: reqMem,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: OTEL_WEBSERVER_CONFIG_DIR,
					Name:      "apache-conf-dir",
				},
				{
					MountPath: OTEL_WEBSERVER_AGENT_DIR,
					Name:      "otel-agent-repo-apache",
				},
			},
		},
	})
	return patchOps
}

func getApacheOtelConfig(pod corev1.Pod, instrRules *InstrumentationRule) string {
	template := `
LoadFile %[1]s/sdk_lib/lib/libopentelemetry_common.so
LoadFile %[1]s/sdk_lib/lib/libopentelemetry_resources.so
LoadFile %[1]s/sdk_lib/lib/libopentelemetry_trace.so
LoadFile %[1]s/sdk_lib/lib/libopentelemetry_otlp_recordable.so
LoadFile %[1]s/sdk_lib/lib/libopentelemetry_exporter_ostream_span.so
LoadFile %[1]s/sdk_lib/lib/libopentelemetry_exporter_otlp_grpc.so

#Load the ApacheModule SDK
LoadFile %[1]s/sdk_lib/lib/libopentelemetry_webserver_sdk.so
#Load the Apache Module. In this example for Apache 2.4
#LoadModule otel_apache_module %[1]s/WebServerModule/Apache/libmod_apache_otel22.so
LoadModule otel_apache_module %[1]s/WebServerModule/Apache/libmod_apache_otel.so
ApacheModuleEnabled ON

#ApacheModule Otel Exporter details
ApacheModuleOtelSpanExporter otlp
ApacheModuleOtelExporterEndpoint %[2]s

# SSL Certificates
#ApacheModuleOtelSslEnabled ON
#ApacheModuleOtelSslCertificatePath 

#ApacheModuleOtelSpanProcessor Batch
#ApacheModuleOtelSampler AlwaysOn
#ApacheModuleOtelMaxQueueSize 1024
#ApacheModuleOtelScheduledDelayMillis 3000
#ApacheModuleOtelExportTimeoutMillis 30000
#ApacheModuleOtelMaxExportBatchSize 1024

ApacheModuleServiceName %[3]s
ApacheModuleServiceNamespace %[4]s
ApacheModuleServiceInstanceId %[5]s

ApacheModuleResolveBackends ON
ApacheModuleTraceAsError ON
#ApacheModuleWebserverContext DemoService DemoServiceNamespace DemoInstanceId

#ApacheModuleSegmentType custom
#ApacheModuleSegmentParameter 15,1,6,7          
`

	collectorEndpoint := ""
	if instrRules.InjectionRules.OpenTelemetryCollector != "" {
		otelCollConfig, found := otelCollsConfig[instrRules.InjectionRules.OpenTelemetryCollector]
		if !found {
			log.Printf("Cannot find OTel collector definition %s\n", instrRules.InjectionRules.OpenTelemetryCollector)
		} else {
			if otelCollConfig.Mode == "sidecar" {
				collectorEndpoint = "http://localhost:4317"
			} else if (otelCollConfig.Mode == "deployment") || (otelCollConfig.Mode == "external") {
				collectorEndpoint = fmt.Sprintf("http://%s:4317", otelCollConfig.ServiceName)
			}
		}
	} else {
		log.Printf("Cannot find OTel collector definition in rule %s\n", instrRules.Name)
	}

	return fmt.Sprintf(template,
		OTEL_WEBSERVER_AGENT_DIR,
		collectorEndpoint,
		getTierName(pod, instrRules),
		getApplicationName(pod, instrRules),
		pod.GetName()+pod.GetGenerateName()+"a")
}

func addApacheApplicationContainerCloneAsInit(pod corev1.Pod, instrRules *InstrumentationRule, containerId int) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	initContainerSpec := pod.Spec.Containers[containerId].DeepCopy()
	initContainerSpec.Name = "apache-source-copy"
	initContainerSpec.Resources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    limCPU,
			corev1.ResourceMemory: limMem,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    reqCPU,
			corev1.ResourceMemory: reqMem,
		},
	}
	initContainerSpec.VolumeMounts = append(initContainerSpec.VolumeMounts,
		corev1.VolumeMount{
			MountPath: OTEL_WEBSERVER_CONFIG_DIR,
			Name:      "apache-conf-dir",
		},
	)
	initContainerSpec.Command = []string{"/bin/sh", "-c"}
	initContainerSpec.Args = []string{"cp -r /usr/local/apache2/conf/* " + OTEL_WEBSERVER_CONFIG_DIR}

	patchOps = append(patchOps, patchOperation{
		Op:    "add",
		Path:  "/spec/initContainers/-",
		Value: initContainerSpec,
	})
	return patchOps
}

func dropApachePassedConfig(pod corev1.Pod, instrRules *InstrumentationRule, containerId int) []patchOperation {
	patchOps := []patchOperation{}

	for idx, volume := range pod.Spec.Containers[containerId].VolumeMounts {
		if strings.Contains(volume.MountPath, "/usr/local/apache2/conf") { // potentially passes config, which we want to pass to init copy only
			patchOps = append(patchOps, patchOperation{
				Op:   "remove",
				Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/%d", containerId, idx),
			})
		}
	}

	return patchOps
}
