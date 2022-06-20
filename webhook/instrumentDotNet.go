package main

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func dotnetInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
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

	patchOps = append(patchOps, addControllerEnvVars(0)...)
	patchOps = append(patchOps, addDotnetEnvVar(pod, instrRule, 0)...)
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_APPLICATION_NAME", getApplicationName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_TIER_NAME", getTierName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_REUSE_NODE_NAME_PREFIX", getTierName(pod, instrRule), 0))

	patchOps = append(patchOps, addNetvizEnvVars(pod, instrRule, 0)...)

	patchOps = append(patchOps, addDotnetAgentVolumeMount(pod, instrRule, 0)...)

	patchOps = append(patchOps, addDotnetAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addDotnetAgentVolume(pod, instrRule)...)

	return patchOps
}

func addDotnetEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, addContainerEnvVar("LD_LIBRARY_PATH", "/opt/appdynamics-dotnetcore", 0))
	patchOps = append(patchOps, addContainerEnvVar("CORECLR_PROFILER", "{57e1aa68-2229-41aa-9931-a6e93bbc64d8}", 0))
	patchOps = append(patchOps, addContainerEnvVar("CORECLR_PROFILER_PATH", "/opt/appdynamics-dotnetcore/libappdprofiler.so", 0))
	patchOps = append(patchOps, addContainerEnvVar("CORECLR_ENABLE_PROFILING", "1", 0))

	return patchOps
}

func addDotnetAgentVolumeMount(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
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

func addDotnetAgentVolume(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
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

func addDotnetAgentInitContainer(pod corev1.Pod, instrRules *InstrumentationRule) []patchOperation {
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
