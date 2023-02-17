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
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TemplateParams struct {
	Labels               map[string]string
	Annotations          map[string]string
	NamespaceLabels      map[string]string
	NamespaceAnnotations map[string]string
	Namespace            string
}

func instrument(pod corev1.Pod, instrRule *InstrumentationRule) ([]patchOperation, error) {

	getADIs(pod.GetNamespace())

	patchOps := []patchOperation{}

	containerIdx := 0

	log.Printf("Using instrumentation rule : %s", instrRule.Name)

	if len(pod.Annotations) == 0 {
		patchOps = append(patchOps, patchOperation{
			Op:    "add",
			Path:  "/metadata/annotations",
			Value: make(map[string]string),
		})
	}

	if len(pod.Spec.Containers) > 0 {
		// fmt.Printf("Container Env: %d -> %v\n", len(pod.Spec.Containers[0].Env), pod.Spec.Containers[0].Env)
		if len(pod.Spec.Containers[containerIdx].Env) == 0 {
			patchOps = append(patchOps, patchOperation{
				Op:    "add",
				Path:  fmt.Sprintf("/spec/containers/%d/env", containerIdx),
				Value: []corev1.EnvVar{},
			})
		}
		if len(pod.Spec.Containers[containerIdx].VolumeMounts) == 0 {
			patchOps = append(patchOps, patchOperation{
				Op:    "add",
				Path:  fmt.Sprintf("/spec/containers/%d/volumeMounts", containerIdx),
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

	if len(instrRule.InjectionRuleSet) > 0 {
		// If injection rule set defined, loop over rules
		// append all together
		for _, injectionRule := range instrRule.InjectionRuleSet {
			instrRule.InjectionRules = &injectionRule
			patchOps = append(patchOps, applyInjectionRule(pod, instrRule)...)

			patchOps = removeDupliciteEnvs(patchOps, containerIdx)
		}
	} else {
		// It's a simple injection rule, one provider, one technology
		patchOps = append(patchOps, applyInjectionRule(pod, instrRule)...)
	}

	return patchOps, nil
}

func applyInjectionRule(pod corev1.Pod, instrRule *InstrumentationRule) []patchOperation {
	patchOps := []patchOperation{}

	_, provider := getTechnologyAndProvider(instrRule.InjectionRules.Technology)

	switch provider {
	case "appd":
		patchOps = append(patchOps, appdInstrumentation(pod, instrRule)...)
	case "telescope":
		patchOps = append(patchOps, telescopeInstrumentation(pod, instrRule)...)
	case "otel":
		patchOps = append(patchOps, otelInstrumentation(pod, instrRule)...)
	}

	return patchOps
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
	case "apache":
		patchOps = append(patchOps, apacheOtelInstrumentation(pod, instrRule)...)
	case "nginx":
		patchOps = append(patchOps, nginxOtelInstrumentation(pod, instrRule)...)
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
	case "namespaceLabel":
		nsName := pod.GetNamespace()
		ns, err := clientset.CoreV1().Namespaces().Get(context.Background(), nsName, metav1.GetOptions{})
		if err != nil {
			log.Printf("Cannot read namespace %s: %v\n", nsName, err)
			appName = nsName
		} else {
			appName = ns.GetLabels()[injRules.ApplicationNameLabel]
		}
	case "namespaceAnnotation":
		nsName := pod.GetNamespace()
		ns, err := clientset.CoreV1().Namespaces().Get(context.Background(), nsName, metav1.GetOptions{})
		if err != nil {
			log.Printf("Cannot read namespace %s: %v\n", nsName, err)
			appName = nsName
		} else {
			appName = ns.GetAnnotations()[injRules.ApplicationNameAnnotation]
		}
	case "expression":
		tmpl, err := template.New("expr").Parse(injRules.ApplicationNameExpression)
		if err != nil {
			log.Printf("Cannot parse application name expresstion %s: %v\n", injRules.ApplicationNameExpression, err)
			appName = "DEFAULT_APP_NAME"
		} else {
			nsName := pod.GetNamespace()
			ns, err := clientset.CoreV1().Namespaces().Get(context.Background(), nsName, metav1.GetOptions{})
			if err != nil {
				log.Printf("Cannot read namespace %s: %v\n", nsName, err)
				appName = nsName
			} else {
				params := TemplateParams{
					Labels:               pod.GetLabels(),
					Annotations:          pod.GetAnnotations(),
					NamespaceLabels:      ns.GetLabels(),
					NamespaceAnnotations: ns.GetAnnotations(),
					Namespace:            pod.GetNamespace(),
				}
				var nameBytes bytes.Buffer
				tmpl.Execute(&nameBytes, params)
				appName = nameBytes.String()
			}
		}
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

func addK8SOtelResourceAttrs(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	preSetOtelResAttrs := ""
	podEnvs := pod.Spec.Containers[containerIdx].Env
	if podEnvs != nil {
		for _, env := range podEnvs {
			if env.Name == "OTEL_RESOURCE_ATTRIBUTES" {
				preSetOtelResAttrs = env.Value
				break
			}
		}
	}

	if *instrRules.InjectionRules.InjectK8SOtelResourceAttrs {
		patchOps = append(patchOps, patchOperation{
			Op:   "add",
			Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
			Value: corev1.EnvVar{
				Name: "K8S_POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
		})

		patchOps = append(patchOps, patchOperation{
			Op:   "add",
			Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
			Value: corev1.EnvVar{
				Name: "K8S_POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		})

		patchOps = append(patchOps, patchOperation{
			Op:   "add",
			Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
			Value: corev1.EnvVar{
				Name: "K8S_NAMESPACE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
		})

		containerName := pod.Spec.Containers[containerIdx].Name
		otelResourceAttributes := "k8s.pod.ip=$(K8S_POD_IP),k8s.pod.name=$(K8S_POD_NAME),k8s.namespace.name=$(K8S_NAMESPACE_NAME)"
		otelResourceAttributes = otelResourceAttributes + ",k8s.container.name=" + containerName
		// TODO - think about getting right number of restarts
		otelResourceAttributes = otelResourceAttributes + ",k8s.container.restart_count=0"

		if preSetOtelResAttrs != "" {
			otelResourceAttributes = preSetOtelResAttrs + "," + otelResourceAttributes
		}

		patchOps = append(patchOps, patchOperation{
			Op:   "add",
			Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
			Value: corev1.EnvVar{
				Name:  "OTEL_RESOURCE_ATTRIBUTES",
				Value: otelResourceAttributes,
			},
		})
	}

	log.Printf("OrelRsrs: %b, %v\n", *instrRules.InjectionRules.InjectK8SOtelResourceAttrs, patchOps)

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

	// this assumes secret exists in a given namespace, at this time, it's not ensured by the
	// webhook!
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

func removeDupliciteEnvs(patchOps []patchOperation, containerIdx int) []patchOperation {

	newPatchOps := []patchOperation{}

	envMap := map[string]int{}

	for idx, po := range patchOps {
		if po.Path == fmt.Sprintf("/spec/containers/%d/env/-", containerIdx) {
			env, ok := po.Value.(corev1.EnvVar)
			if !ok {
				continue
			}
			envMap[env.Name] = idx
		}
	}

	log.Printf("Env Map: %v\n", envMap)

	for idx, po := range patchOps {
		if po.Path != fmt.Sprintf("/spec/containers/%d/env/-", containerIdx) {
			newPatchOps = append(newPatchOps, po)
		} else {
			env, ok := po.Value.(corev1.EnvVar)
			if !ok { // should not ever happen, but just to be sure...
				newPatchOps = append(newPatchOps, po)
				continue
			}
			if envMap[env.Name] == idx { //this is the last occurence, use it
				newPatchOps = append(newPatchOps, po)
			}
		}
	}

	return newPatchOps
}

func getADIs(namespace string) error {
	adiGVR := schema.GroupVersionResource{
		Group:    "ext.appd.com",
		Version:  "v1",
		Resource: "appdynamicsinstrumentations",
	}
	adis, err := client.Resource(adiGVR).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Cannot get ADIs in namespace %s - %v\n", namespace, err)
		return err
	}

	for _, adi := range adis.Items {
		log.Printf("ADI - %s - %v\n", adi.GetName(), adi)
		spec := adi.UnstructuredContent()["spec"].(map[string]interface{})
		log.Printf("ADI Spec: \nexclude: %v\ninclude: %v\n", spec["exclude"], spec["include"])
	}

	return err
}
