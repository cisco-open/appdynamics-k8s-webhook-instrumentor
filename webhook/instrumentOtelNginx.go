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

func nginxOtelInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}

	containerId := 0

	patchOps = append(patchOps, addOtelNginxEnvVar(pod, instrRule, containerId)...)
	patchOps = append(patchOps, addSpecifiedContainerEnvVars(instrRule.InjectionRules.EnvVars, containerId)...)

	patchOps = append(patchOps, addOtelNginxAgentVolumeMount(pod, instrRule, containerId)...)
	patchOps = append(patchOps, addNginxApplicationContainerCloneAsInit(pod, instrRule, containerId)...)
	patchOps = append(patchOps, dropNginxPassedConfig(pod, instrRule, containerId)...)
	patchOps = append(patchOps, addOtelNginxAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addOtelNginxAgentVolume(pod, instrRule)...)
	patchOps = append(patchOps, addOtelNginxSourceConfVolume(pod, instrRule)...)

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

func addOtelNginxEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
		Value: corev1.EnvVar{
			Name:  "LD_LIBRARY_PATH",
			Value: OTEL_WEBSERVER_AGENT_DIR + "/sdk_lib/lib",
		},
	})

	return patchOps
}

func addOtelNginxAgentVolumeMount(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	// directory with modified Apache conf directory
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: "/etc/nginx",
			Name:      "nginx-conf-dir",
		},
	})
	// directory with webserver agent
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: OTEL_WEBSERVER_AGENT_DIR, //TODO
			Name:      "otel-agent-repo-nginx",  //TODO
		},
	})
	return patchOps
}

func addOtelNginxAgentVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "otel-agent-repo-nginx", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addOtelNginxSourceConfVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "nginx-conf-dir", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addOtelNginxAgentInitContainer(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	////////////////////////////////////////////////////////
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/initContainers/-",
		Value: corev1.Container{
			Name:    "otel-agent-attach-nginx", //TODO
			Image:   instrRules.InjectionRules.Image,
			Command: []string{"/bin/sh", "-c"},
			Args: []string{
				"cp -ar /opt/opentelemetry/* " + OTEL_WEBSERVER_AGENT_DIR + " && " +
					"export agentLogDir=$(echo \"" + OTEL_WEBSERVER_AGENT_DIR + "/logs\" | sed 's,/,\\\\/,g') && " +
					"cat " + OTEL_WEBSERVER_AGENT_DIR + "/conf/appdynamics_sdk_log4cxx.xml.template | sed 's/__agent_log_dir__/'${agentLogDir}'/g'  > " + OTEL_WEBSERVER_AGENT_DIR + "/conf/appdynamics_sdk_log4cxx.xml &&" +
					"echo \"$OPENTELEMETRY_MODULE_CONF\" > " + OTEL_WEBSERVER_CONFIG_DIR + "/opentelemetry_agent.conf && " +
					"sed -i '1s,^,load_module " + OTEL_WEBSERVER_AGENT_DIR + "/WebServerModule/Nginx/ngx_http_opentelemetry_module.so;\\n,g' " + OTEL_WEBSERVER_CONFIG_DIR + "/nginx.conf && " +
					"cp " + OTEL_WEBSERVER_CONFIG_DIR + "/opentelemetry_agent.conf " + OTEL_WEBSERVER_CONFIG_DIR + "/conf.d",

				// "cp " + OTEL_WEBSERVER_CONFIG_DIR + "/nginx.conf " + OTEL_WEBSERVER_CONFIG_DIR + "/nginx.conf.old && " +
				// Patch nginx.conf - include agent libs + conf
				// OTEL_WEBSERVER_AGENT_DIR + "/nginxConfPatcher " + OTEL_WEBSERVER_CONFIG_DIR + "/nginx.conf.old " +
				// OTEL_WEBSERVER_AGENT_DIR + "/WebServerModule/Nginx/ngx_http_opentelemetry_module.so " +
				// "/etc/nginx/opentelemetry_agent.conf > " + OTEL_WEBSERVER_CONFIG_DIR + "/nginx.conf",
			},
			ImagePullPolicy: corev1.PullAlways,
			Env: []corev1.EnvVar{
				{
					Name:  "OPENTELEMETRY_MODULE_CONF",
					Value: getNginxOtelConfig(pod, instrRules),
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
					Name:      "nginx-conf-dir",
				},
				{
					MountPath: OTEL_WEBSERVER_AGENT_DIR,
					Name:      "otel-agent-repo-nginx",
				},
			},
		},
	})
	return patchOps
}

func getNginxOtelConfig(pod corev1.Pod, instrRules *InstrumentationRule) string {
	template := `
NginxModuleEnabled ON;
NginxModuleOtelSpanExporter otlp;
NginxModuleOtelExporterEndpoint %[1]s;
NginxModuleServiceName %[2]s;
NginxModuleServiceNamespace %[3]s;
NginxModuleServiceInstanceId %[4]s;
NginxModuleResolveBackends ON;
NginxModuleTraceAsError ON;
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
		collectorEndpoint,
		getTierName(pod, instrRules),
		getApplicationName(pod, instrRules),
		pod.GetName()+pod.GetGenerateName()+"a")
}

func addNginxApplicationContainerCloneAsInit(pod corev1.Pod, instrRules *InstrumentationRule, containerId int) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	initContainerSpec := pod.Spec.Containers[containerId].DeepCopy()
	initContainerSpec.Name = "nginx-source-copy"
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
			Name:      "nginx-conf-dir",
		},
	)
	initContainerSpec.Command = []string{"/bin/sh", "-c"}
	initContainerSpec.Args = []string{"cp -r /etc/nginx/* " + OTEL_WEBSERVER_CONFIG_DIR}

	patchOps = append(patchOps, patchOperation{
		Op:    "add",
		Path:  "/spec/initContainers/-",
		Value: initContainerSpec,
	})
	return patchOps
}

func dropNginxPassedConfig(pod corev1.Pod, instrRules *InstrumentationRule, containerId int) []patchOperation {
	patchOps := []patchOperation{}

	for idx, volume := range pod.Spec.Containers[containerId].VolumeMounts {
		if strings.Contains(volume.MountPath, "/etc/nginx") { // potentially passes config, which we want to pass to init copy only
			patchOps = append(patchOps, patchOperation{
				Op:   "remove",
				Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/%d", containerId, idx),
			})
		}
	}

	return patchOps
}
