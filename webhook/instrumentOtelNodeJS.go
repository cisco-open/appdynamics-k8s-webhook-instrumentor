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

func nodejsOtelInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
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

	patchOps = append(patchOps, addOtelNodejsEnvVar(pod, instrRule, 0)...)
	patchOps = append(patchOps, addContainerEnvVar("OTEL_SERVICE_NAMESPACE", getApplicationName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("OTEL_SERVICE_NAME", getTierName(pod, instrRule), 0))

	patchOps = append(patchOps, addOtelNodejsAgentVolumeMount(pod, instrRule, 0)...)

	patchOps = append(patchOps, addOtelNodejsAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addOtelNodejsAgentVolume(pod, instrRule)...)

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

func addOtelNodejsEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, addContainerEnvVar("NODE_OPTIONS", "--require /opt/opentelemetry-agent/shim.js", 0))

	return patchOps
}

func addOtelNodejsAgentVolumeMount(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: "/opt/opentelemetry-agent", //TODO
			Name:      "otel-agent-repo-nodejs",   //TODO
		},
	})
	return patchOps
}

func addOtelNodejsAgentVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "otel-agent-repo-nodejs", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addOtelNodejsAgentInitContainer(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/initContainers/-",
		Value: corev1.Container{
			Name:            "otel-agent-attach-nodejs", //TODO
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
				Name:      "otel-agent-repo-nodejs",   //TODO
			}},
		},
	})
	return patchOps
}
