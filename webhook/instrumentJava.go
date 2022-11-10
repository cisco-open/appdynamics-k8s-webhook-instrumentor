package main

import corev1 "k8s.io/api/core/v1"

func javaInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
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
	patchOps = append(patchOps, addJavaEnvVar(pod, instrRule, 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_APPLICATION_NAME", getApplicationName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_TIER_NAME", getTierName(pod, instrRule), 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_REUSE_NODE_NAME_PREFIX", getTierName(pod, instrRule), 0))

	patchOps = append(patchOps, addNetvizEnvVars(pod, instrRule, 0)...)

	patchOps = append(patchOps, addJavaAgentVolumeMount(pod, instrRule, 0)...)

	patchOps = append(patchOps, addJavaAgentInitContainer(pod, instrRule)...)

	patchOps = append(patchOps, addJavaAgentVolume(pod, instrRule)...)

	return patchOps
}
