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
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func instrument(pod corev1.Pod, instrRule *InstrumentationRule) ([]patchOperation, error) {

	patchOps := []patchOperation{}

	log.Printf("Using instrumentation rule : %s", instrRule.Name)

	if len(pod.Annotations) == 0 {
		patchOps = append(patchOps, patchOperation{
			Op:    "add",
			Path:  "/metadata/annotations",
			Value: make(map[string]string),
		})
	}

	_, provider := getTechnologyAndProvider(instrRule.InjectionRules.Technology)

	switch provider {
	case "appd":
		patchOps = append(patchOps, appdInstrumentation(pod, instrRule)...)
	case "telescope":
		patchOps = append(patchOps, telescopeInstrumentation(pod, instrRule)...)
	case "otel":
		patchOps = append(patchOps, otelInstrumentation(pod, instrRule)...)
	}

	return patchOps, nil
}

func appdInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {

	patchOps := []patchOperation{}

	patchOps = append(patchOps, patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations/APPD_INSTRUMENTATION_VIA_RULE",
		Value: string(instrRule.Name),
	})

	technology, _ := getTechnologyAndProvider(instrRule.InjectionRules.Technology)

	switch technology {
	case "java":
		patchOps = append(patchOps, javaAppdInstrumentation(pod, instrRule)...)
	case "dotnetcore":
		patchOps = append(patchOps, dotnetAppdInstrumentation(pod, instrRule)...)
	case "nodejs":
		patchOps = append(patchOps, nodejsAppdInstrumentation(pod, instrRule)...)
	case "apache":
		patchOps = append(patchOps, apacheAppdInstrumentation(pod, instrRule)...)
	default:
		patchOps = append(patchOps, getInstrumentationStatusPatch("FAILED", "Technology for injection not specified or unknown")...)
	}

	return patchOps
}

func telescopeInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {

	patchOps := []patchOperation{}

	patchOps = append(patchOps, patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations/TELESCOPE_INSTRUMENTATION_VIA_RULE",
		Value: string(instrRule.Name),
	})

	technology, _ := getTechnologyAndProvider(instrRule.InjectionRules.Technology)

	switch technology {
	case "java":
		patchOps = append(patchOps, javaTelescopeInstrumentation(pod, instrRule)...)
	case "nodejs":
		patchOps = append(patchOps, nodejsTelescopeInstrumentation(pod, instrRule)...)
	default:
		patchOps = append(patchOps, getInstrumentationStatusPatch("FAILED", "Technology for injection not specified or unknown")...)
	}

	return patchOps
}

func otelInstrumentation(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {

	patchOps := []patchOperation{}

	patchOps = append(patchOps, patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations/OTEL_INSTRUMENTATION_VIA_RULE",
		Value: string(instrRule.Name),
	})

	technology, _ := getTechnologyAndProvider(instrRule.InjectionRules.Technology)

	switch technology {
	case "java":
		patchOps = append(patchOps, javaOtelInstrumentation(pod, instrRule)...)
	case "dotnetcore":
		patchOps = append(patchOps, dotnetOtelInstrumentation(pod, instrRule)...)
	case "nodejs":
		patchOps = append(patchOps, nodejsOtelInstrumentation(pod, instrRule)...)
	default:
		patchOps = append(patchOps, getInstrumentationStatusPatch("FAILED", "Technology for injection not specified or unknown")...)
	}

	return patchOps
}

func getApplicationName(pod corev1.Pod, instrRule *InstrumentationRule) string {
	appName := ""
	injRules := instrRule.InjectionRules
	switch injRules.ApplicationNameSource {
	case "manual":
		appName = injRules.ApplicationName
	case "label":
		appName = pod.GetLabels()[injRules.ApplicationNameLabel]
	case "annotation":
		appName = pod.GetAnnotations()[injRules.ApplicationNameAnnotation]
	case "namespace":
		appName = pod.GetNamespace()
	default:
		appName = "DEFAULT_APP_NAME"
	}
	return appName
}

func getTierName(pod corev1.Pod, instrRule *InstrumentationRule) string {
	tierName := ""
	injRules := instrRule.InjectionRules
	switch injRules.TierNameSource {
	case "auto":
		if len(pod.GetOwnerReferences()) > 0 {
			or := pod.GetOwnerReferences()[0]
			switch or.Kind {
			case "ReplicaSet", "ReplicationController":
				nameElems := strings.Split(or.Name, "-")
				tierName = strings.Join(nameElems[0:len(nameElems)-1], "-")
			default:
				tierName = or.Name
			}
		} else {
			tierName = pod.GetName()
		}
	case "manual":
		tierName = injRules.TierName
	case "label":
		tierName = pod.GetLabels()[injRules.TierNameLabel]
	case "annotation":
		tierName = pod.GetAnnotations()[injRules.TierNameAnnotation]
	case "namespace":
		tierName = pod.GetNamespace()
	default:
		tierName = "DEFAULT_TIER_NAME"
	}
	return tierName
}

func getInstrumentationStatusPatch(status string, reason string) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations/APPD_INSTRUMENTATION_STATUS",
		Value: status,
	})
	if reason != "" {
		patchOps = append(patchOps, patchOperation{
			Op:    "add",
			Path:  "/metadata/annotations/APPD_INSTRUMENTATION_FAILURE_REASON",
			Value: reason,
		})
	}

	return patchOps
}

func addContainerEnvVar(name string, value string, containerIdx int) patchOperation {
	return patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
		Value: corev1.EnvVar{
			Name:  name,
			Value: value,
		},
	}
}

func addSpecifiedContainerEnvVars(vars []NameValue, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	for _, envvar := range vars {
		patchOps = append(patchOps, patchOperation{
			Op:   "add",
			Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
			Value: corev1.EnvVar{
				Name:  envvar.Name,
				Value: envvar.Value,
			},
		})
	}
	return patchOps
}

func addNetvizEnvVars(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	patchOps = append(patchOps, patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
		Value: corev1.EnvVar{
			Name: "APPDYNAMICS_NETVIZ_AGENT_HOST",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "status.hostIP",
				},
			},
		},
	})
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_NETVIZ_AGENT_PORT", instrRules.InjectionRules.NetvizPort, 0))

	return patchOps
}

func addControllerEnvVars(containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	if config.ControllerConfig.AccessKeySecret != "" {
		patchOps = append(patchOps, patchOperation{
			Op:   "add",
			Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
			Value: corev1.EnvVar{
				Name: "APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: config.ControllerConfig.AccessKeySecretKey,
						LocalObjectReference: corev1.LocalObjectReference{
							Name: config.ControllerConfig.AccessKeySecret,
						},
					},
				},
			},
		})
	} else {
		patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY", config.ControllerConfig.AccessKey, 0))
	}
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_CONTROLLER_HOST_NAME", config.ControllerConfig.Host, 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_CONTROLLER_PORT", config.ControllerConfig.Port, 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_CONTROLLER_SSL_ENABLED", strconv.FormatBool(config.ControllerConfig.IsSecure), 0))
	patchOps = append(patchOps, addContainerEnvVar("APPDYNAMICS_AGENT_ACCOUNT_NAME", config.ControllerConfig.AccountName, 0))

	return patchOps
}

func addTemplate(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	return patchOps
}

func reuseNodeNames(instrRules *InstrumentationRule) bool {
	if instrRules.InjectionRules.UsePodNameForNodeName != nil {
		if *instrRules.InjectionRules.UsePodNameForNodeName {
			return false
		}
	}
	return true
}

func getTechnologyAndProvider(technologyString string) (string, string) {
	technology := ""
	provider := "appd"
	elems := strings.Split(technologyString, "/")
	if len(elems) == 1 {
		technology = elems[0]
	} else {
		technology = elems[0]
		provider = elems[1]
	}
	return technology, provider
}
