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

func nodejsTelescopeInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, addTelescopeNodejsEnvVar(pod, instrRule, 0)...)
	patchOps = append(patchOps, addContainerEnvVar("OTEL_SERVICE_NAMESPACE", getApplicationName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("OTEL_SERVICE_NAME", getTierName(pod, instrRule), 0))

	patchOps = append(patchOps, addSpecifiedContainerEnvVars(instrRule.InjectionRules.EnvVars, 0)...)

	patchOps = append(patchOps, addTelescopeNodejsAgentVolumeMount(pod, instrRule, 0)...)

	patchOps = append(patchOps, addTelescopeNodejsAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addTelescopeNodejsAgentVolume(pod, instrRule)...)

	if instrRule.InjectionRules.OpenTelemetryCollector != "" {
		otelCollConfig, found := otelCollsConfig[instrRule.InjectionRules.OpenTelemetryCollector]
		if !found {
			log.Printf("Cannot find OTel collector definition %s\n", instrRule.InjectionRules.OpenTelemetryCollector)
		} else {
			if otelCollConfig.Mode == "sidecar" {
				patchOps = append(patchOps, addContainerEnvVar("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318/v1/traces", 0))
				patchOps = append(patchOps, addOtelCollSidecar(pod, instrRule, 0)...)
			} else if (otelCollConfig.Mode == "deployment") || (otelCollConfig.Mode == "external") {
				patchOps = append(patchOps, addContainerEnvVar("OTEL_EXPORTER_OTLP_ENDPOINT", fmt.Sprintf("http://%s:4318/v1/traces", otelCollConfig.ServiceName), 0))
			}
		}
	}
	return patchOps
}

func addTelescopeNodejsEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, addContainerEnvVar("NODE_OPTIONS", "--require /opt/telescope-agent/shim.js", 0))

	return patchOps
}

func addTelescopeNodejsAgentVolumeMount(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: "/opt/telescope-agent",        //TODO
			Name:      "telescope-agent-repo-nodejs", //TODO
		},
	})
	return patchOps
}

func addTelescopeNodejsAgentVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "telescope-agent-repo-nodejs", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addTelescopeNodejsAgentInitContainer(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/initContainers/-",
		Value: corev1.Container{
			Name:            "telescope-agent-attach-nodejs", //TODO
			Image:           instrRules.InjectionRules.Image,
			Command:         []string{"cp", "-r", "/opt/telescope/.", "/opt/telescope-agent"},
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
				MountPath: "/opt/telescope-agent",        //TODO
				Name:      "telescope-agent-repo-nodejs", //TODO
			}},
		},
	})
	return patchOps
}
