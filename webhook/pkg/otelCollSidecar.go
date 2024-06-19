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
	"v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const OTEL_COLL_CONFIG_MAP_NAME = "otel-collector-config"
const DEFAULT_OTEL_COLLECTOR_IMAGE = "otel/opentelemetry-collector-contrib:latest"

type OtelCollConfig struct {
	Config          string
	Mode            string
	Image           string
	ImagePullPolicy string
	InitImage       string
	ServiceName     string
	OtelColSpec     v1alpha1.OpenTelemetryCollectorSpec
}

var otelCollsConfig = map[string]OtelCollConfig{}
var otelCollsConfigNamespaced = map[string]map[string]OtelCollConfig{}
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

func addOtelCollSidecar(pod corev1.Pod, instrRules *v1alpha1.InstrumentationSpec, containerIdx int) []patchOperation {
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

		otelCollConfig, namespaced, err := getCollectorConfigsByName(pod.GetNamespace(), instrRules.InjectionRules.OpenTelemetryCollector)
		if err != nil {
			log.Printf("Cannot find OTel collector definition %v\n", err)
			return []patchOperation{}
		}

		if namespaced {
			patchOps = append(patchOps, addOtelCollSidecarNamespaced(pod, instrRules, containerIdx)...)
			return patchOps
		}

		// else use the configmap based configuration, for the time being, less sophisticated
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

func getCollectorConfigsByName(namespace string, otelCollName string) (*OtelCollConfig, bool, error) {
	// first check, if there's a match in namespaced collectors
	otelCollsInNamespace, found := otelCollsConfigNamespaced[namespace]
	if found {
		otelCollConfig, found := otelCollsInNamespace[otelCollName]
		if found {
			return &otelCollConfig, true, nil
		}
	}

	// if not, search the collectors provided by the instrumentor
	otelCollConfig, found := otelCollsConfig[otelCollName]
	if !found {
		return nil, false, fmt.Errorf("cannot find OTel collector definition %s", otelCollName)
	} else {
		return &otelCollConfig, false, nil
	}
}

func addOtelCollSidecarNamespaced(pod corev1.Pod, instrRules *v1alpha1.InstrumentationSpec, containerIdx int) []patchOperation {
	patchOps := []patchOperation{}

	otelCollConfig, namespaced, err := getCollectorConfigsByName(pod.GetNamespace(), instrRules.InjectionRules.OpenTelemetryCollector)
	if err != nil || !namespaced {
		log.Printf("Cannot find namespaced (%t) OTel collector definition %v\n", namespaced, err)
		return []patchOperation{}
	}

	// here we rely on CRD-based otel col spec. In future, this should be changed for config map-based
	// otel col as well
	otelCollSpec := otelCollConfig.OtelColSpec

	if otelCollSpec.Mode != v1alpha1.ModeSidecar {
		log.Printf("Sidecar OTEL collector has invalid mode %s\n", otelCollSpec.Mode)
		return []patchOperation{}

	}

	if otelCollSpec.Image == "" {
		otelCollSpec.Image = DEFAULT_OTEL_COLLECTOR_IMAGE
	}

	limCPUInit, _ := resource.ParseQuantity("200m")
	limMemInit, _ := resource.ParseQuantity("75M")
	reqCPUInit, _ := resource.ParseQuantity("10m")
	reqMemInit, _ := resource.ParseQuantity("50M")

	sidecar := corev1.Container{
		Name:            "otel-coll-sidecar",
		Image:           otelCollSpec.Image,
		Args:            []string{"--config", "/conf/otel-collector-config.yaml"},
		ImagePullPolicy: corev1.PullPolicy(otelCollSpec.ImagePullPolicy),
		Ports: []corev1.ContainerPort{
			{Name: "otlp-grpc", ContainerPort: 4317},
			{Name: "otlp-http", ContainerPort: 4318},
		},
		Resources: otelCollSpec.Resources,
		VolumeMounts: append(otelCollSpec.VolumeMounts, corev1.VolumeMount{
			MountPath: "/conf",
			Name:      "otel-collector-config-vol",
		}),
		Env:     otelCollSpec.Env,
		EnvFrom: otelCollSpec.EnvFrom,
	}

	sidecarInit := corev1.Container{
		Name:            "otel-coll-sidecar-init",
		Image:           OTELCOL_CONFIG_INJECTOR_IMAGE,
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
			Value: otelCollSpec.Config,
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

	for _, vol := range otelCollSpec.Volumes {
		patchOps = append(patchOps, patchOperation{
			Op:    "add",
			Path:  "/spec/volumes/-",
			Value: vol,
		})
	}

	return patchOps
}

func registerNamespacedSidecarCollector(namespace string, collector *v1alpha1.OpenTelemetryCollector) {
	collectors := map[string]OtelCollConfig{}
	found := false
	if collectors, found = otelCollsConfigNamespaced[namespace]; !found {
		collectors = map[string]OtelCollConfig{}
		otelCollsConfigNamespaced[namespace] = collectors
	}

	name := collector.GetName()
	newCollectorConfig := OtelCollConfig{
		Image:           collector.Spec.Image,
		ImagePullPolicy: string(collector.Spec.ImagePullPolicy),
		Mode:            string(collector.Spec.Mode),
		InitImage:       OTELCOL_CONFIG_INJECTOR_IMAGE,
		ServiceName:     "",
		Config:          collector.Spec.Config,
		OtelColSpec:     collector.Spec,
	}

	collectors[name] = newCollectorConfig
	otelCollsConfigNamespaced[namespace] = collectors
}

func unregisterNamespacedCollector(namespace string, name string) {
	if collectors, found := otelCollsConfigNamespaced[namespace]; found {
		delete(collectors, name)
		otelCollsConfigNamespaced[namespace] = collectors
	}
}

func registerNamespacedStandaloneCollector(namespace string, collector *v1alpha1.OpenTelemetryCollector) {
	collectors := map[string]OtelCollConfig{}
	found := false
	if collectors, found = otelCollsConfigNamespaced[namespace]; !found {
		collectors = map[string]OtelCollConfig{}
		otelCollsConfigNamespaced[namespace] = collectors
	}

	serviceName := OTELCOL_RESOURCE_PREFIX + collector.GetName() + "." + namespace + ".svc.cluster.local"
	if collector.Spec.Mode == v1alpha1.ModeExternal {
		serviceName = collector.Spec.OtlpEndpoint
	}

	name := collector.GetName()
	newCollectorConfig := OtelCollConfig{
		Image:           collector.Spec.Image,
		ImagePullPolicy: string(collector.Spec.ImagePullPolicy),
		Mode:            string(collector.Spec.Mode),
		InitImage:       OTELCOL_CONFIG_INJECTOR_IMAGE,
		ServiceName:     serviceName,
		Config:          collector.Spec.Config,
		OtelColSpec:     collector.Spec,
	}

	collectors[name] = newCollectorConfig
	otelCollsConfigNamespaced[namespace] = collectors
}
