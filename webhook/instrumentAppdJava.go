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

func javaAppdInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, addControllerEnvVars(0)...)
	patchOps = append(patchOps, addJavaEnvVar(pod, instrRule, 0)...)
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_APPLICATION_NAME", getApplicationName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_TIER_NAME", getTierName(pod, instrRule), 0))
	if reuseNodeNames(instrRule) {
		patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_REUSE_NODE_NAME_PREFIX", getTierName(pod, instrRule), 0))
	}

	patchOps = append(patchOps, addSpecifiedContainerEnvVars(instrRule.InjectionRules.EnvVars, 0)...)

	patchOps = append(patchOps, addNetvizEnvVars(pod, instrRule, 0)...)

	patchOps = append(patchOps, addJavaAgentVolumeMount(pod, instrRule, 0)...)

	patchOps = append(patchOps, addJavaAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addJavaAgentVolume(pod, instrRule)...)

	if instrRule.InjectionRules.OpenTelemetryCollector != "" {
		otelCollConfig, found := otelCollsConfig[instrRule.InjectionRules.OpenTelemetryCollector]
		if !found {
			log.Printf("Cannot find OTel collector definition %s\n", instrRule.InjectionRules.OpenTelemetryCollector)
		} else {
			if otelCollConfig.Mode == "sidecar" {
				patchOps = append(patchOps, addOtelCollSidecar(pod, instrRule, 0)...)
			}
		}
	}

	return patchOps
}

func addJavaEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
		Value: corev1.EnvVar{
			Name:  instrRules.InjectionRules.JavaEnvVar,
			Value: getJavaOptions(pod, instrRules),
		},
	})

	if !reuseNodeNames(instrRules) {
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

	return patchOps
}

func getJavaOptions(pod corev1.Pod, instrRules *InstrumentationRule) string {
	javaOpts := " "

	if config.ControllerConfig.UseProxy {
		javaOpts += fmt.Sprintf("-Dappdynamics.http.proxyHost=%s ", config.ControllerConfig.ProxyHost)
		javaOpts += fmt.Sprintf("-Dappdynamics.http.proxyPort=%s ", config.ControllerConfig.ProxyPort)
	}

	javaOpts += "-Dappdynamics.agent.accountAccessKey=$(APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY) "
	if reuseNodeNames(instrRules) {
		javaOpts += "-Dappdynamics.agent.reuse.nodeName=true "
	}
	javaOpts += "-Dappdynamics.socket.collection.bci.enable=true "
	javaOpts += "-javaagent:/opt/appdynamics-java/javaagent.jar "
	javaOpts += instrRules.InjectionRules.JavaCustomConfig + " "

	// OpenTelemetry Java Options for AppD hybrid agent
	if instrRules.InjectionRules.OpenTelemetryCollector != "" {
		otelCollConfig, found := otelCollsConfig[instrRules.InjectionRules.OpenTelemetryCollector]
		if !found {
			log.Printf("Cannot find OTel collector definition %s\n", instrRules.InjectionRules.OpenTelemetryCollector)
		} else {
			javaOpts += "-Dappdynamics.opentelemetry.enabled=true "
			javaOpts += fmt.Sprintf("-Dotel.resource.attributes=service.name=%s,service.namespace=%s ", getTierName(pod, instrRules), getApplicationName(pod, instrRules))
			javaOpts += "-Dotel.traces.exporter=otlp,logging "
			if otelCollConfig.Mode == "sidecar" {
				javaOpts += "-Dotel.exporter.otlp.traces.endpoint=http://localhost:4317 "
			} else if (otelCollConfig.Mode == "deployment") || (otelCollConfig.Mode == "external") {
				javaOpts += fmt.Sprintf("-Dotel.exporter.otlp.traces.endpoint=http://%s:4317 ", otelCollConfig.ServiceName)
			}
		}
	}

	return javaOpts
}

func addJavaAgentVolumeMount(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: "/opt/appdynamics-java", //TODO
			Name:      "appd-agent-repo-java",  //TODO
		},
	})
	return patchOps
}

func addJavaAgentVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "appd-agent-repo-java", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addJavaAgentInitContainer(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/initContainers/-",
		Value: corev1.Container{
			Name:            "appd-agent-attach-java", //TODO
			Image:           instrRules.InjectionRules.Image,
			Command:         []string{"cp", "-r", "/opt/appdynamics/.", "/opt/appdynamics-java"},
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
				MountPath: "/opt/appdynamics-java", //TODO
				Name:      "appd-agent-repo-java",  //TODO
			}},
		},
	})
	return patchOps
}
