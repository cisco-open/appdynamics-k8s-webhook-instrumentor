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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func dotnetOtelInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}

	//	patchOps = append(patchOps, addControllerEnvVars(0)...)
	patchOps = append(patchOps, addOtelDotnetEnvVar(pod, instrRule, 0)...)

	patchOps = append(patchOps, addContainerEnvVar("OTEL_SERVICE_NAMESPACE", getApplicationName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("OTEL_SERVICE_NAME", getTierName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("OTEL_RESOURCE_ATTRIBUTES",
		fmt.Sprintf("service.name=%s,service.namespace=%s", getTierName(pod, instrRule), getApplicationName(pod, instrRule)), 0))

	patchOps = append(patchOps, addContainerEnvVar("OTEL_TRACES_EXPORTER", "otlp", 0))

	if instrRule.InjectionRules.OpenTelemetryCollector != "" {
		otelCollConfig, found := otelCollsConfig[instrRule.InjectionRules.OpenTelemetryCollector]
		if !found {
			log.Printf("Cannot find OTel collector definition %s\n", instrRule.InjectionRules.OpenTelemetryCollector)
		} else {
			if otelCollConfig.Mode == "sidecar" {
				patchOps = append(patchOps, addContainerEnvVar("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318", 0))
				patchOps = append(patchOps, addOtelCollSidecar(pod, instrRule, 0)...)
			} else if (otelCollConfig.Mode == "deployment") || (otelCollConfig.Mode == "external") {
				patchOps = append(patchOps, addContainerEnvVar("OTEL_EXPORTER_OTLP_ENDPOINT", fmt.Sprintf("http://%s:4318", otelCollConfig.ServiceName), 0))
			}
		}
	}

	patchOps = append(patchOps, addSpecifiedContainerEnvVars(instrRule.InjectionRules.EnvVars, 0)...)

	patchOps = append(patchOps, addOtelDotnetAgentVolumeMount(pod, instrRule, 0)...)

	patchOps = append(patchOps, addOtelDotnetAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addOtelDotnetAgentVolume(pod, instrRule)...)

	return patchOps
}

func addOtelDotnetEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	/*
	 * COR_ENABLE_PROFILING=1
	 * COR_PROFILER={918728DD-259F-4A6A-AC2B-B85E1B658318}
	 * CORECLR_ENABLE_PROFILING=1
	 * CORECLR_PROFILER={918728DD-259F-4A6A-AC2B-B85E1B658318}
	 * DOTNET_ADDITIONAL_DEPS=%InstallationLocation%/AdditionalDeps
	 * DOTNET_SHARED_STORE=%InstallationLocation%/store
	 * DOTNET_STARTUP_HOOKS=%InstallationLocation%/netcoreapp3.1/OpenTelemetry.AutoInstrumentation.StartupHook.dll
	 * OTEL_DOTNET_AUTO_HOME=%InstallationLocation%
	 * OTEL_DOTNET_AUTO_INTEGRATIONS_FILE=%InstallationLocation%/integrations.json
	 * CORECLR_PROFILER_PATH=%InstallationLocation%/OpenTelemetry.AutoInstrumentation.Native.so

	 OTEL_DOTNET_AUTO_TRACES_ENABLED
	 OTEL_DOTNET_AUTO_METRICS_ENABLED

	*/

	AGENT_PATH := "/opt/opentelemetry-agent"
	patchOps = append(patchOps, addContainerEnvVar("OTEL_DOTNET_AUTO_TRACES_ENABLED", "true", 0))
	patchOps = append(patchOps, addContainerEnvVar("OTEL_DOTNET_AUTO_METRICS_ENABLED", "false", 0))

	patchOps = append(patchOps, addContainerEnvVar("CORECLR_ENABLE_PROFILING", "1", 0))
	patchOps = append(patchOps, addContainerEnvVar("CORECLR_PROFILER", "{918728DD-259F-4A6A-AC2B-B85E1B658318}", 0))
	patchOps = append(patchOps, addContainerEnvVar("COR_ENABLE_PROFILING", "1", 0))
	patchOps = append(patchOps, addContainerEnvVar("COR_PROFILER", "{918728DD-259F-4A6A-AC2B-B85E1B658318}", 0))
	patchOps = append(patchOps, addContainerEnvVar("DOTNET_ADDITIONAL_DEPS", fmt.Sprintf("%s/AdditionalDeps", AGENT_PATH), 0))
	patchOps = append(patchOps, addContainerEnvVar("DOTNET_SHARED_STORE", fmt.Sprintf("%s/store", AGENT_PATH), 0))
	patchOps = append(patchOps, addContainerEnvVar("DOTNET_STARTUP_HOOKS", fmt.Sprintf("%s/netcoreapp3.1/OpenTelemetry.AutoInstrumentation.StartupHook.dll", AGENT_PATH), 0))
	patchOps = append(patchOps, addContainerEnvVar("OTEL_DOTNET_AUTO_HOME", fmt.Sprintf("%s", AGENT_PATH), 0))
	patchOps = append(patchOps, addContainerEnvVar("OTEL_DOTNET_AUTO_INTEGRATIONS_FILE", fmt.Sprintf("%s/integrations.json", AGENT_PATH), 0))
	patchOps = append(patchOps, addContainerEnvVar("CORECLR_PROFILER_PATH", fmt.Sprintf("%s/OpenTelemetry.AutoInstrumentation.Native.so", AGENT_PATH), 0))

	return patchOps
}

func addOtelDotnetAgentVolumeMount(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: "/opt/opentelemetry-agent",   //TODO
			Name:      "otel-agent-repo-dotnetcore", //TODO
		},
	})
	return patchOps
}

func addOtelDotnetAgentVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "otel-agent-repo-dotnetcore", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addOtelDotnetAgentInitContainer(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/initContainers/-",
		Value: corev1.Container{
			Name:            "otel-agent-attach-dotnetcore", //TODO
			Image:           instrRules.InjectionRules.Image,
			Command:         []string{"cp", "-r", "/opt/opentelemetry/.", "/opt/opentelemetry-agent"},
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
				MountPath: "/opt/opentelemetry-agent",   //TODO
				Name:      "otel-agent-repo-dotnetcore", //TODO
			}},
		},
	})
	return patchOps
}
