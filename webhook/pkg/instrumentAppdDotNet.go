/*
Copyright (c) 2019 Cisco Systems, Inc.

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
	"v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func dotnetAppdInstrumentation(pod corev1.Pod, instrRule *v1alpha1.InstrumentationSpec) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, addControllerEnvVars(0)...)
	patchOps = append(patchOps, addDotnetEnvVar(pod, instrRule, 0)...)
	patchOps = append(patchOps, addDotnetOtelEnvVar(pod, instrRule, 0)...)
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_APPLICATION_NAME", getApplicationName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_TIER_NAME", getTierName(pod, instrRule), 0))
	if reuseNodeNames(instrRule) {
		patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_REUSE_NODE_NAME_PREFIX", getTierName(pod, instrRule), 0))
	}

	patchOps = append(patchOps, addSpecifiedContainerEnvVars(instrRule.InjectionRules.EnvVars, 0)...)

	patchOps = append(patchOps, addNetvizEnvVars(pod, instrRule, 0)...)

	patchOps = append(patchOps, addDotnetAgentVolumeMount(pod, instrRule, 0)...)

	patchOps = append(patchOps, addDotnetAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addDotnetAgentVolume(pod, instrRule)...)

	return patchOps
}

func addDotnetEnvVar(pod corev1.Pod, instrRules *v1alpha1.InstrumentationSpec, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, addContainerEnvVar("LD_LIBRARY_PATH", "/opt/appdynamics-dotnetcore", 0))
	patchOps = append(patchOps, addContainerEnvVar("CORECLR_PROFILER", "{57e1aa68-2229-41aa-9931-a6e93bbc64d8}", 0))
	patchOps = append(patchOps, addContainerEnvVar("CORECLR_PROFILER_PATH", "/opt/appdynamics-dotnetcore/libappdprofiler.so", 0))
	patchOps = append(patchOps, addContainerEnvVar("CORECLR_ENABLE_PROFILING", "1", 0))
	if reuseNodeNames(instrRules) {
		patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_REUSE_NODE_NAME", "true", 0))
	} else {
		patchOps = append(patchOps, patchOperation{
			Op:   "add",
			Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
			Value: corev1.EnvVar{
				Name: "APPDYNAMICS_AGENT_NODE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
		})
	}

	if config.ControllerConfig.UseProxy {
		patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_PROXY_HOST_NAME", config.ControllerConfig.ProxyHost, 0))
		patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_PROXY_PORT", config.ControllerConfig.ProxyPort, 0))
		if config.ControllerConfig.ProxyUser != "" {
			patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_PROXY_AUTH_NAME", config.ControllerConfig.ProxyUser, 0))
			patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_PROXY_AUTH_PASSWORD", config.ControllerConfig.ProxyPassword, 0))
		}
		if config.ControllerConfig.ProxyDomain != "" {
			patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_PROXY_AUTH_DOMAIN", config.ControllerConfig.ProxyDomain, 0))
		}
	}

	return patchOps
}

func addDotnetOtelEnvVar(pod corev1.Pod, instrRule *v1alpha1.InstrumentationSpec, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	if instrRule.InjectionRules.OpenTelemetryCollector != "" {
		otelCollConfig, _, err := getCollectorConfigsByName(pod.GetNamespace(), instrRule.InjectionRules.OpenTelemetryCollector)
		// otelCollConfig, found := otelCollsConfig[instrRule.InjectionRules.OpenTelemetryCollector]
		if err != nil {
			log.Printf("Cannot find OTel collector definition %s\n", instrRule.InjectionRules.OpenTelemetryCollector)
		} else {

			patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_OPENTELEMETRY_ENABLED", "true", containerIdx))
			patchOps = append(patchOps, addContainerEnvVar("OTEL_TRACES_EXPORTER", "otlp", containerIdx))
			otelRsrcAttrs := ""
			if *instrRule.InjectionRules.InjectK8SOtelResourceAttrs {
				patchOps = append(patchOps, addK8SOtelResourceAttrs(pod, instrRule, containerIdx, "OTEL_RESOURCE_ATTRIBUTES_K8S")...)
				otelRsrcAttrs = ",$(OTEL_RESOURCE_ATTRIBUTES_K8S)"
			}
			resourceAttributes := fmt.Sprintf("service.name=%s,service.namespace=%s%s", getTierName(pod, instrRule), getApplicationName(pod, instrRule), otelRsrcAttrs)
			patchOps = append(patchOps, addContainerEnvVar("OTEL_RESOURCE_ATTRIBUTES", resourceAttributes, containerIdx))
			if otelCollConfig.Mode == "sidecar" {
				patchOps = append(patchOps, addContainerEnvVar("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318", containerIdx))
				patchOps = append(patchOps, addOtelCollSidecar(pod, instrRule, containerIdx)...)
			} else if (otelCollConfig.Mode == "deployment") || (otelCollConfig.Mode == "external") {
				patchOps = append(patchOps, addContainerEnvVar("OTEL_EXPORTER_OTLP_ENDPOINT", fmt.Sprintf("http://%s:4318", otelCollConfig.ServiceName), containerIdx))
			}
		}
	}

	return patchOps
}

func addDotnetAgentVolumeMount(pod corev1.Pod, instrRules *v1alpha1.InstrumentationSpec, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: "/opt/appdynamics-dotnetcore", //TODO
			Name:      "appd-agent-repo-dotnetcore",  //TODO
		},
	})
	return patchOps
}

func addDotnetAgentVolume(pod corev1.Pod, instrRules *v1alpha1.InstrumentationSpec) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "appd-agent-repo-dotnetcore", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addDotnetAgentInitContainer(pod corev1.Pod, instrRules *v1alpha1.InstrumentationSpec) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/initContainers/-",
		Value: corev1.Container{
			Name:            "appd-agent-attach-dotnetcore", //TODO
			Image:           instrRules.InjectionRules.Image,
			Command:         []string{"cp", "-r", "/opt/appdynamics/.", "/opt/appdynamics-dotnetcore"},
			ImagePullPolicy: corev1.PullAlways, //TODO
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
			VolumeMounts: []corev1.VolumeMount{{
				MountPath: "/opt/appdynamics-dotnetcore", //TODO
				Name:      "appd-agent-repo-dotnetcore",  //TODO
			}},
		},
	})
	return patchOps
}
