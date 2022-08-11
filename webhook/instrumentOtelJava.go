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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func javaOtelInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}

	if len(pod.Spec.Containers) > 0 {
		// fmt.Printf("Container Env: %d -> %v\n", len(pod.Spec.Containers[0].Env), pod.Spec.Containers[0].Env)
		if len(pod.Spec.Containers[0].Env) == 0 {
			patchOps = append(patchOps, patchOperation{
				Op:    "add",
				Path:  "/spec/containers/0/env",
				Value: []corev1.EnvVar{},
			})
		}
		if len(pod.Spec.Containers[0].VolumeMounts) == 0 {
			patchOps = append(patchOps, patchOperation{
				Op:    "add",
				Path:  "/spec/containers/0/volumeMounts",
				Value: []corev1.VolumeMount{},
			})
		}
		if len(pod.Spec.Volumes) == 0 {
			patchOps = append(patchOps, patchOperation{
				Op:    "add",
				Path:  "/spec/volumes/",
				Value: []corev1.Volume{},
			})
		}
		if len(pod.Spec.InitContainers) == 0 {
			patchOps = append(patchOps, patchOperation{
				Op:    "add",
				Path:  "/spec/initContainers",
				Value: []corev1.VolumeMount{},
			})
		}
	}

	patchOps = append(patchOps, addOtelJavaEnvVar(pod, instrRule, 0)...)
	patchOps = append(patchOps, addContainerEnvVar("OTEL_TRACES_EXPORTER", "otlp", 0))
	patchOps = append(patchOps, addContainerEnvVar("OTEL_RESOURCE_ATTRIBUTES",
		fmt.Sprintf("service.name=%s,service.namespace=%s", getTierName(pod, instrRule), getApplicationName(pod, instrRule)), 0))
	patchOps = append(patchOps, addContainerEnvVar("OTEL_SERVICE_NAME", getTierName(pod, instrRule), 0))

	patchOps = append(patchOps, addSpecifiedContainerEnvVars(instrRule.InjectionRules.EnvVars, 0)...)

	patchOps = append(patchOps, addOtelJavaAgentVolumeMount(pod, instrRule, 0)...)

	patchOps = append(patchOps, addOtelJavaAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addOtelJavaAgentVolume(pod, instrRule)...)

	if instrRule.InjectionRules.OpenTelemetryCollector != "" {
		otelCollConfig, found := otelCollsConfig[instrRule.InjectionRules.OpenTelemetryCollector]
		if !found {
			log.Printf("Cannot find OTel collector definition %s\n", instrRule.InjectionRules.OpenTelemetryCollector)
		} else {
			if otelCollConfig.Mode == "sidecar" {
				patchOps = append(patchOps, addContainerEnvVar("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317", 0))
				patchOps = append(patchOps, addOtelCollSidecar(pod, instrRule, 0)...)
			} else if (otelCollConfig.Mode == "deployment") || (otelCollConfig.Mode == "external") {
				patchOps = append(patchOps, addContainerEnvVar("OTEL_EXPORTER_OTLP_ENDPOINT", fmt.Sprintf("http://%s:4317", otelCollConfig.ServiceName), 0))
			}
		}
	}

	return patchOps
}

func addOtelJavaEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
		Value: corev1.EnvVar{
			Name:  instrRules.InjectionRules.JavaEnvVar,
			Value: getOtelJavaOptions(pod, instrRules),
		},
	})

	return patchOps
}

func getOtelJavaOptions(pod corev1.Pod, instrRules *InstrumentationRule) string {
	javaOpts := " "

	if config.ControllerConfig.UseProxy {
		javaOpts += fmt.Sprintf("-Dappdynamics.http.proxyHost=%s ", config.ControllerConfig.ProxyHost)
		javaOpts += fmt.Sprintf("-Dappdynamics.http.proxyPort=%s ", config.ControllerConfig.ProxyPort)
	}

	javaOpts += "-javaagent:/opt/opentelemetry-agent/opentelemetry-javaagent.jar "
	javaOpts += instrRules.InjectionRules.JavaCustomConfig + " "

	return javaOpts
}

func addOtelJavaAgentVolumeMount(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: "/opt/opentelemetry-agent", //TODO
			Name:      "otel-agent-repo-java",     //TODO
		},
	})
	return patchOps
}

func addOtelJavaAgentVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "otel-agent-repo-java", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addOtelJavaAgentInitContainer(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/initContainers/-",
		Value: corev1.Container{
			Name:            "otel-agent-attach-java", //TODO
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
				MountPath: "/opt/opentelemetry-agent", //TODO
				Name:      "otel-agent-repo-java",     //TODO
			}},
		},
	})
	return patchOps
}
