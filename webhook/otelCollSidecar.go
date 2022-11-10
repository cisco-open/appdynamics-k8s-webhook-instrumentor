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
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const OTEL_COLL_CONFIG_MAP_NAME = "otel-collector-config"

type OtelCollConfig struct {
	Config          string
	Mode            string
	Image           string
	ImagePullPolicy string
	InitImage       string
	ServiceName     string
}

var otelCollsConfig = map[string]OtelCollConfig{}
var otelCollsConfigMutex = sync.Mutex{}

func loadOtelConfig(cm map[string]string) {
	otelCollsConfigMutex.Lock()
	defer otelCollsConfigMutex.Unlock()

	for key, value := range cm {
		keyElems := strings.Split(key, ".")
		collectorName := keyElems[0]
		itemKey := strings.Join(keyElems[1:], ".")
		if _, found := otelCollsConfig[collectorName]; !found {
			otelCollsConfig[collectorName] = OtelCollConfig{}
		}
		collectorConfig := otelCollsConfig[collectorName]
		switch itemKey {
		case "config":
			collectorConfig.Config = value
		case "mode":
			collectorConfig.Mode = value
		case "image.image":
			collectorConfig.Image = value
		case "image.imagePullPolicy":
			collectorConfig.ImagePullPolicy = value
		case "image.initImage":
			collectorConfig.InitImage = value
		case "serviceName":
			collectorConfig.ServiceName = value
		}
		otelCollsConfig[collectorName] = collectorConfig
	}
}

func addOtelCollSidecar(pod corev1.Pod, instrRules *InstrumentationRule, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	if len(pod.Spec.Containers) > 0 {
		// TODO - lookup the otel template for instrumentation rule
		limCPU, _ := resource.ParseQuantity("1")
		reqCPU, _ := resource.ParseQuantity("200m")
		limMem, _ := resource.ParseQuantity("1Gi")
		reqMem, _ := resource.ParseQuantity("200Mi")

		limCPUInit, _ := resource.ParseQuantity("300m")
		reqCPUInit, _ := resource.ParseQuantity("50m")
		limMemInit, _ := resource.ParseQuantity("200Mi")
		reqMemInit, _ := resource.ParseQuantity("100Mi")

		otelCollConfig, err := getCollectorConfigsByName(instrRules.InjectionRules.OpenTelemetryCollector)
		if err != nil {
			log.Printf("Cannot find OTel collector definition %v\n", err)
			return []patchOperation{}
		}

		sidecar := corev1.Container{
			Name:            "otel-coll-sidecar",
			Image:           otelCollConfig.Image,
			Args:            []string{"--config", "/conf/otel-collector-config.yaml"},
			ImagePullPolicy: corev1.PullPolicy(otelCollConfig.ImagePullPolicy),
			Ports:           []corev1.ContainerPort{{Name: "otlp-grpc", ContainerPort: 4317}, {Name: "otlp-http", ContainerPort: 4318}},
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
				MountPath: "/conf",
				Name:      "otel-collector-config-vol",
			}},
		}

		sidecarInit := corev1.Container{
			Name:            "otel-coll-sidecar-init",
			Image:           otelCollConfig.InitImage,
			Command:         []string{"/bin/sh", "-c"},
			Args:            []string{"echo \"$OTEL_COLL_CONFIG\" > /conf/otel-collector-config.yaml"},
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    limCPUInit,
					corev1.ResourceMemory: limMemInit,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    reqCPUInit,
					corev1.ResourceMemory: reqMemInit,
				},
			},
			VolumeMounts: []corev1.VolumeMount{{
				MountPath: "/conf",
				Name:      "otel-collector-config-vol",
			}},
			Env: []corev1.EnvVar{{
				Name:  "OTEL_COLL_CONFIG",
				Value: otelCollConfig.Config,
			}},
		}

		configVolume := corev1.Volume{
			Name: "otel-collector-config-vol",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}

		patchOps = append(patchOps, patchOperation{
			Op:    "add",
			Path:  "/spec/containers/-",
			Value: sidecar,
		})

		patchOps = append(patchOps, patchOperation{
			Op:    "add",
			Path:  "/spec/initContainers/-",
			Value: sidecarInit,
		})

		patchOps = append(patchOps, patchOperation{
			Op:    "add",
			Path:  "/spec/volumes/-",
			Value: configVolume,
		})
	}

	return patchOps
}

func getCollectorConfigsByName(otelCollName string) (*OtelCollConfig, error) {
	otelCollConfig, found := otelCollsConfig[otelCollName]
	if !found {
		return nil, fmt.Errorf("cannot find OTel collector definition %s", otelCollName)
	} else {
		return &otelCollConfig, nil
	}
}
