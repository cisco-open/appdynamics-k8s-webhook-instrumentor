// Copyright Cisco Systems
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +kubebuilder:validation:Required
package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&OpenTelemetryCollector{}, &OpenTelemetryCollectorList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=otelcol;otelcols
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.scale.replicas,selectorpath=.status.scale.selector
// +kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.mode",description="Deployment Mode"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenTelemetry Collector"
// This annotation provides a hint for OLM which resources are managed by OpenTelemetryCollector kind.
// It's not mandatory to list all resources.
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1},{Deployment,apps/v1},{DaemonSets,apps/v1},{StatefulSets,apps/v1},{ConfigMaps,v1},{Service,v1}}

// OpenTelemetryCollector is the Schema for the opentelemetrycollectors API.
type OpenTelemetryCollector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenTelemetryCollectorSpec   `json:"spec,omitempty"`
	Status OpenTelemetryCollectorStatus `json:"status,omitempty"`
}

func (*OpenTelemetryCollector) Hub() {}

//+kubebuilder:object:root=true

// OpenTelemetryCollectorList contains a list of OpenTelemetryCollector.
type OpenTelemetryCollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenTelemetryCollector `json:"items"`
}

// OpenTelemetryCollectorStatus defines the observed state of OpenTelemetryCollector.
type OpenTelemetryCollectorStatus struct {
}

// OpenTelemetryCollectorSpec defines the desired state of OpenTelemetryCollector.
type OpenTelemetryCollectorSpec struct {
	// Resources to set on generated pods.
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// Args is the set of arguments to pass to the main container's binary.
	// +optional
	Args map[string]string `json:"args,omitempty"`
	// Replicas is the number of pod instances for the underlying replicaset. Set this if you are not using autoscaling.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// ServiceAccount indicates the name of an existing service account to use with this instance. When set,
	// the operator will not automatically create a ServiceAccount.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image indicates the container image to use for the generated pods.
	// +optional
	Image string `json:"image,omitempty"`
	// ImagePullPolicy indicates the pull policy to be used for retrieving the container image.
	// +optional
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// VolumeMounts represents the mount points to use in the underlying deployment(s).
	// +optional
	// +listType=atomic
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// Ports allows a set of ports to be exposed by the underlying v1.Service. By default, the operator
	// will attempt to infer the required ports by parsing the .Spec.Config property but this property can be
	// used to open additional ports that can't be inferred by the operator, like for custom receivers.
	// +optional
	// +listType=atomic
	Ports []v1.ServicePort `json:"ports,omitempty"`
	// Environment variables to set on the generated pods.
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
	// List of sources to populate environment variables on the generated pods.
	// +optional
	EnvFrom []v1.EnvFromSource `json:"envFrom,omitempty"`
	// Volumes represents which volumes to use in the underlying deployment(s).
	// +optional
	// +listType=atomic
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// Mode represents how the collector should be deployed (deployment or sidecar)
	// +optional
	// +kubebuilder:validation:Enum=deployment;sidecar;external
	Mode Mode `json:"mode,omitempty"`
	// Config is the raw JSON to be used as the collector's configuration. Refer to the OpenTelemetry Collector documentation for details.
	// The empty objects e.g. batch: should be written as batch: {} otherwise they won't work with kustomize or kubectl edit.
	// +required
	Config string `json:"config"`
	// OtlpEndpoint is the hostname of an OpenTelemetry collector running independently from the instrumentor tool.
	// Used only for Mode == ModeExternal
	// +optional
	OtlpEndpoint string `json:"otlpEndpoint,omitempty"`
}

type (
	// Mode represents how the collector should be deployed (deployment vs. sidecar)
	// +kubebuilder:validation:Enum=deployment;sidecar
	Mode string
)

const (
	// ModeDeployment specifies that the collector should be deployed as a Kubernetes Deployment.
	ModeDeployment Mode = "deployment"

	// ModeSidecar specifies that the collector should be deployed as a sidecar to pods.
	ModeSidecar Mode = "sidecar"

	// ModeExternal specifies that the collector already exists independently at a given address.
	ModeExternal Mode = "external"
)
