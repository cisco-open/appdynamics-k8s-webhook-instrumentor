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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func nodejsAppdInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, addControllerEnvVars(0)...)
	patchOps = append(patchOps, addNodejsEnvVar(pod, instrRule, 0)...)
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_APPLICATION_NAME", getApplicationName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_TIER_NAME", getTierName(pod, instrRule), 0))
	if reuseNodeNames(instrRule) {
		patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_REUSE_NODE_NAME_PREFIX", getTierName(pod, instrRule), 0))
	}

	// not sure it has to be there, but ClusterAgent does the following, too
	// patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_NODE_NAME", getTierName(pod, instrRule), 0))

	patchOps = append(patchOps, addSpecifiedContainerEnvVars(instrRule.InjectionRules.EnvVars, 0)...)

	patchOps = append(patchOps, addNetvizEnvVars(pod, instrRule, 0)...)

	patchOps = append(patchOps, addNodejsAgentVolumeMount(pod, instrRule, 0)...)

	patchOps = append(patchOps, addNodejsAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addNodejsAgentVolume(pod, instrRule)...)

	return patchOps
}

func addNodejsEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, addContainerEnvVar("NODE_OPTIONS", "--require /opt/appdynamics-nodejs/shim.js", 0))
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

	// Check for proxy settings, doc does not say anything
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

func addNodejsAgentVolumeMount(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIdx),
		Value: corev1.VolumeMount{
			MountPath: "/opt/appdynamics-nodejs", //TODO
			Name:      "appd-agent-repo-nodejs",  //TODO
		},
	})
	return patchOps
}

func addNodejsAgentVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/volumes/-",
		Value: corev1.Volume{
			Name: "appd-agent-repo-nodejs", //TODO
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	})
	return patchOps
}

func addNodejsAgentInitContainer(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}
	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.CPU)
	reqMem, _ := resource.ParseQuantity(instrRules.InjectionRules.ResourceReservation.Memory)

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: "/spec/initContainers/-",
		Value: corev1.Container{
			Name:            "appd-agent-attach-nodejs", //TODO
			Image:           instrRules.InjectionRules.Image,
			Command:         []string{"cp", "-r", "/opt/appdynamics/.", "/opt/appdynamics-nodejs"},
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
				MountPath: "/opt/appdynamics-nodejs", //TODO
				Name:      "appd-agent-repo-nodejs",  //TODO
			}},
		},
	})
	return patchOps
}
