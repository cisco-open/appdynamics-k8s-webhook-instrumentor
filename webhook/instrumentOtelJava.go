package main

import corev1 "k8s.io/api/core/v1"

func javaOtelInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}

	return patchOps
}
