package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

	patchOps = append(patchOps, patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations/APPD_INSTRUMENTATION_VIA_RULE",
		Value: string(instrRule.Name),
	})

	switch instrRule.InjectionRules.Technology {
	case "java":
		patchOps = append(patchOps, javaInstrumentation(pod, instrRule)...)
	case "dotnetcore":
		patchOps = append(patchOps, dotnetInstrumentation(pod, instrRule)...)
	case "nodejs":
		patchOps = append(patchOps, nodejsInstrumentation(pod, instrRule)...)
	case "apache":
		patchOps = append(patchOps, apacheInstrumentation(pod, instrRule)...)
	default:
		patchOps = append(patchOps, getInstrumentationStatusPatch("FAILED", "Technology for injection not specified or unknown")...)
	}

	return patchOps, nil
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

func addJavaEnvVar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) patchOperation {
	return patchOperation{
		Op:   "add",
		Path: fmt.Sprintf("/spec/containers/%d/env/-", containerIdx),
		Value: corev1.EnvVar{
			Name:  instrRules.InjectionRules.JavaEnvVar,
			Value: getJavaOptions(pod, instrRules),
		},
	}
}

func getJavaOptions(pod corev1.Pod, instrRules *InstrumentationRule) string {
	javaOpts := " "

	javaOpts += "-Dappdynamics.agent.accountAccessKey=$(APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY) "
	javaOpts += "-Dappdynamics.agent.reuse.nodeName=true "
	javaOpts += "-Dappdynamics.socket.collection.bci.enable=true "
	javaOpts += "-javaagent:/opt/appdynamics-java/javaagent.jar "
	javaOpts += instrRules.InjectionRules.JavaCustomConfig

	return javaOpts
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

func addTemplate(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}
	return patchOps
}
