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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MatchRule specifies criteria by which to match workload for instrumentation
type MatchRule struct {
	// Regex by which to match namespace of the workload. Used only for ClusterInstrumentation.
	// +optional
	NamespaceRegex string `json:"namespaceRegex,omitempty" yaml:"namespaceRegex,omitempty"`

	// List of labels and their regex values to match
	// +optional
	Labels *[]map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// List of annotations and their regex values to match
	// +optional
	Annotations *[]map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`

	// Regex to match names of pods
	// +optional
	PodNameRegex string `json:"podNameRegex,omitempty" yaml:"podNameRegex,omitempty"`
}

type InjectionRule struct {
	Template string `json:"template,omitempty" yaml:"template,omitempty"`

	// The programming language or solution to instrument
	// +kubebuilder:validation:Enum=java;dotnetcore;nodejs;apache;nginx;java/appd;dotnetcore/appd;nodejs/appd;apache/appd;nginx/appd;java/otel;dotnetcore/otel;nodejs/otel;apache/otel;nginx/otel;
	Technology       string `json:"technology,omitempty" yaml:"technology,omitempty"`
	Image            string `json:"image,omitempty" yaml:"image,omitempty"`
	JavaEnvVar       string `json:"javaEnvVar,omitempty" yaml:"javaEnvVar,omitempty"`
	JavaCustomConfig string `json:"javaCustomConfig,omitempty" yaml:"javaCustomConfig,omitempty"`

	// Source of AppDynamics application name
	// +kubebuilder:validation:Enum=manual;label;annotation;namespace;namespaceLabel;namespaceAnnotation;expression
	ApplicationNameSource     string `json:"applicationNameSource,omitempty" yaml:"applicationNameSource,omitempty"` // manual,namespace,label,annotation,expression
	ApplicationName           string `json:"applicationName,omitempty" yaml:"applicationName,omitempty"`
	ApplicationNameLabel      string `json:"applicationNameLabel,omitempty" yaml:"applicationNameLabel,omitempty"`
	ApplicationNameAnnotation string `json:"applicationNameAnnotation,omitempty" yaml:"applicationNameAnnotation,omitempty"`
	ApplicationNameExpression string `json:"applicationNameExpression,omitempty" yaml:"applicationNameExpression,omitempty"`

	// Source of AppDynamics tier name
	// +kubebuilder:validation:Enum=auto;manual;label;annotation;namespace
	TierNameSource             string               `json:"tierNameSource,omitempty" yaml:"tierNameSource,omitempty"` // auto,manual,namespace,label,annotation,expression
	TierName                   string               `json:"tierName,omitempty" yaml:"tierName,omitempty"`
	TierNameLabel              string               `json:"tierNameLabel,omitempty" yaml:"tierNameLabel,omitempty"`
	TierNameAnnotation         string               `json:"tierNameAnnotation,omitempty" yaml:"tierNameAnnotation,omitempty"`
	TierNameExpression         string               `json:"tierNameExpression,omitempty" yaml:"tierNameExpression,omitempty"`
	UsePodNameForNodeName      *bool                `json:"usePodNameForNodeName,omitempty" yaml:"usePodNameForNodeName,omitempty"`
	DoNotInstrument            *bool                `json:"doNotInstrument,omitempty" yaml:"doNotInstrument,omitempty"`
	ResourceReservation        *ResourceReservation `json:"resourceReservation,omitempty" yaml:"resourceReservation,omitempty"`
	LogLevel                   string               `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	NetvizPort                 string               `json:"netvizPort,omitempty" yaml:"netvizPort,omitempty"`
	OpenTelemetryCollector     string               `json:"openTelemetryCollector,omitempty" yaml:"openTelemetryCollector,omitempty"`
	EnvVars                    []NameValue          `json:"env,omitempty" yaml:"env,omitempty"`
	Options                    []NameValue          `json:"options,omitempty" yaml:"options,omitempty"`
	InjectK8SOtelResourceAttrs *bool                `json:"injectK8SOtelResourceAttrs,omitempty" yaml:"injectK8SOtelResourceAttrs,omitempty"`
}

type NameValue struct {
	// Variable name
	// +mandatory
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Variable value
	// +mandatory
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}

type ResourceReservation struct {
	// CPU reservation value
	// +mandatory
	CPU string `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	// Memory allocation reservation value
	// +mandatory
	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"`
}

// Instrumentation defines how to inject agent into workload.
type InstrumentationSpec struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Match rule matches the workload for injection
	// +mandatory
	MatchRules *MatchRule `json:"matchRules,omitempty" yaml:"matchRules,omitempty"`

	// Injection rule specifies how the instrumentation should be done
	// +mandatory
	InjectionRules *InjectionRule `json:"injectionRules,omitempty" yaml:"injectionRules,omitempty"`

	// Priority defines priority of this rule - 1 is lowest
	// +kubebuilder:default=1
	Priority int `json:"priority" yaml:"priority"`

	InjectionRuleSet []InjectionRule `json:"injectionRuleSet,omitempty" yaml:"injectionRuleSet,omitempty"`
}

type InjectionTemplate struct {
	Name           string         `json:"name,omitempty" yaml:"name,omitempty" `
	InjectionRules *InjectionRule `json:"injectionRules,omitempty" yaml:"injectionRules,omitempty" `
}

func init() {
	SchemeBuilder.Register(
		&Instrumentation{},
		&ClusterInstrumentation{},
		&InstrumentationList{},
		&ClusterInstrumentationList{},
	)
}

// InstrumentationStatus defines status of the instrumentation.
type InstrumentationStatus struct {
}

// ClusterInstrumentationStatus defines status of the instrumentation.
type ClusterInstrumentationStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=instr;instrs
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="AppDynamics Instrumentation Rule"
// +operator-sdk:csv:customresourcedefinitions:resources={{Instrumentation,v1alpha/ext.appd.com}}

type Instrumentation struct {
	Status            InstrumentationStatus `json:"status,omitempty"`
	metav1.TypeMeta   `json:",inline"`
	Spec              InstrumentationSpec `json:"spec,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// InstrumentationList contains a list of Instrumentation.
// +kubebuilder:object:root=true
type InstrumentationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instrumentation `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=clinstr;clinstrs
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="AppDynamics Cluster Instrumentation Rule"
// +operator-sdk:csv:customresourcedefinitions:resources={{ClusterInstrumentation,v1alpha/ext.appd.com}}

type ClusterInstrumentation struct {
	Status            ClusterInstrumentationStatus `json:"status,omitempty"`
	metav1.TypeMeta   `json:",inline"`
	Spec              InstrumentationSpec `json:"spec,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// InstrumentationList contains a list of Instrumentation.
// +kubebuilder:object:root=true
type ClusterInstrumentationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterInstrumentation `json:"items"`
}
