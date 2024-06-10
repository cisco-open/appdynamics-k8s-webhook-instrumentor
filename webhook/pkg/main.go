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
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	v1alpha1 "v1alpha1"

	admission "k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme *runtime.Scheme = runtime.NewScheme()
)

func init() {
	log.SetLogger(zap.New())
	v1alpha1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	appsv1.AddToScheme(scheme)

}

func (s OtelConfig) String() string {
	return fmt.Sprintf("Trace: %t, Endpoint: %s, Samples/M %d, LogPayload: %t, ServiceName: %s, ServiceNamespace: %s",
		s.Trace, s.Endpoint, s.SamplesPerMillion, s.LogPayload, s.ServiceName, s.ServiceNamespace)
}

const (
	tlsDir      = `/run/secrets/tls`
	tlsCertFile = `tls.crt`
	tlsKeyFile  = `tls.key`
)

var (
	podResource         = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
	nsInstrResource     = metav1.GroupVersionResource{Version: "v1alpha1", Resource: "instrumentations", Group: "ext.appd.com"}
	globalInstrResource = metav1.GroupVersionResource{Version: "v1alpha1", Resource: "clusterinstrumentations", Group: "ext.appd.com"}
	otelCollResource    = metav1.GroupVersionResource{Version: "v1alpha1", Resource: "opentelemetrycollectors", Group: "ext.appd.com"}
)

func handleInstrumentationCRDs(req *admission.AdmissionRequest) ([]patchOperation, error) {
	if req.DryRun != nil {
		if *req.DryRun {
			return []patchOperation{}, nil
		}
	}

	if req.Resource == nsInstrResource {
		log.Log.Info("Validating and registering namespaced instrumentation", "namespace", req.Namespace, "name", req.Name)
		// Parse the Instrumentation object.
		raw := req.Object.Raw
		instr := v1alpha1.Instrumentation{}
		if _, _, err := universalDeserializer.Decode(raw, nil, &instr); err != nil {
			return nil, fmt.Errorf("could not deserialize Instrumentation object: %v", err)
		}

		injectionRuleDefaults(instr.Spec.InjectionRules)
		upsertCrdInstrumentation(req.Namespace, instr.Name, instr.Spec)

	} else if req.Resource == globalInstrResource {
		log.Log.Info("Validating and registering cluster-wide instrumentation", "name", req.Name)
		// Parse the GlobalInstrumentation object.
		raw := req.Object.Raw
		instr := v1alpha1.ClusterInstrumentation{}
		if _, _, err := universalDeserializer.Decode(raw, nil, &instr); err != nil {
			return nil, fmt.Errorf("could not deserialize ClusterInstrumentation object: %v", err)
		}

		injectionRuleDefaults(instr.Spec.InjectionRules)
		upsertCrdClusterInstrumentation(instr.Name, instr.Spec)

	} else if req.Resource == otelCollResource {
		log.Log.Info("Validating and registering OpenTelemetry collector", "namespace", req.Namespace, "name", req.Name)
		// Parse the OpenTelemetryCollector object.
		raw := req.Object.Raw
		otelcol := v1alpha1.OpenTelemetryCollector{}
		if _, _, err := universalDeserializer.Decode(raw, nil, &otelcol); err != nil {
			return nil, fmt.Errorf("could not deserialize OpenTelemetryCollector object: %v", err)
		}

		// if sidecar definition, which is only config as such, register it for later use
		// when instrumented pods are instatiated
		if otelcol.Spec.Mode == v1alpha1.ModeSidecar {
			registerNamespacedSidecarCollector(req.Namespace, &otelcol)
		} else {
			// now it get's tricky - we'll leave collector instantiation for controller reconciler and just register
			// the definition for immediate pod use
			registerNamespacedStandaloneCollector(req.Namespace, &otelcol)
		}

	}

	// TODO - for each type, reconciler should be added to check if in say 10 seconds, resource was really persisted
	// and if not, removed from instrumentation rules

	return []patchOperation{}, nil
}

func applyAppdInstrumentation(req *admission.AdmissionRequest) ([]patchOperation, error) {
	// This handler should only get called on Pod objects as per the MutatingWebhookConfiguration in the YAML file.
	// However, if (for whatever reason) this gets invoked on an object of a different kind, issue a log message but
	// let the object request pass through otherwise.

	// fmt.Println(instrumentationAsString())
	// time.Sleep(1 * time.Second)

	if req.Resource != podResource {
		log.Log.Info("expect resource to be", "pod", podResource)
		return nil, nil
	}

	// Parse the Pod object.
	raw := req.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &pod); err != nil {
		return nil, fmt.Errorf("could not deserialize pod object: %v", err)
	}

	// Check if pod fields are empty but we can fill from other sources
	if len(pod.Name) == 0 && len(pod.GenerateName) > 0 {
		pod.Name = pod.GenerateName
	}

	if len(pod.Namespace) == 0 && len(req.Namespace) > 0 {
		pod.Namespace = req.Namespace
	}

	// Check if we have configuration sucessfully
	if config.ControllerConfig == nil || config.InstrumentationConfig == nil {
		return nil, fmt.Errorf("instrumentor configuration not read from configmap")
	}

	log.Log.Info("Checking instrumentation for", "pod", pod.Name)
	instrumentationRule := getInstrumentationRule(pod)

	if instrumentationRule == nil { // pod not eligible for AppDynamics instrumentation
		return []patchOperation{}, nil
	}

	log.Log.Info("Found instrumentation rule", "rule", instrumentationRule.Name)

	config.mutex.Lock()
	defer config.mutex.Unlock()

	// at this time, pod does not have metadata.namespace assigned
	// but we need it. since this does not get propagated anywhere
	// it's supplied here into the pod data
	pod.Namespace = req.Namespace
	patches, err := instrument(pod, instrumentationRule)

	return patches, err
}

func main() {

	otelTracing := flag.Bool("otel-tracing", false, "set to true to otel traces enabled")
	otelEndpoint := flag.String("otel-endpoint", "localhost:4317", "otel collector endpoint <host>:<port>")
	otelSamplesPerMillion := flag.Int64("otel-samples-per-million", 1000, "number of otel trace samples per million requests")
	otelLogPayload := flag.Bool("otel-log-layload", false, "set to true if payload attached to traces as attribute")
	otelServiceName := flag.String("otel-service-name", "mwh", "service name")
	otelServiceNamespace := flag.String("otel-service-namespace", "default", "service namespace")
	flag.Parse()

	otelConfig = OtelConfig{
		Trace:             *otelTracing,
		Endpoint:          *otelEndpoint,
		SamplesPerMillion: *otelSamplesPerMillion,
		LogPayload:        *otelLogPayload,
		ServiceName:       *otelServiceName,
		ServiceNamespace:  *otelServiceNamespace,
	}

	if otelConfig.Trace {
		log.Log.Info("Using otel tracing", "tracing", otelConfig)
		shutdown, err := initOtelTracing()
		if err != nil {
			log.Log.Info("Error initializing OTEL tracing", "error", err)
		} else {
			defer shutdown()
		}
	}

	go runConfigWatcher()

	// select {}
	// return

	certPath := filepath.Join(tlsDir, tlsCertFile)
	keyPath := filepath.Join(tlsDir, tlsKeyFile)

	tracer := getTracer("webhook-tracer")
	log.Log.Info("Otel Tracer", "tracer", tracer)

	go startCrdReconciler()

	mux := http.NewServeMux()

	mux.Handle("/mutate", otelHandler(admitFuncHandler(applyAppdInstrumentation), "/mutate"))
	mux.Handle("/validate", otelHandler(admitFuncHandler(handleInstrumentationCRDs), "/validate"))
	mux.Handle("/api/config", otelHandler(configHandler(), "/api/config"))
	server := &http.Server{
		// We listen on port 8443 such that we do not need root privileges or extra capabilities for this server.
		// The Service object will take care of mapping this port to the HTTPS port 443.
		Addr:    ":8443",
		Handler: mux,
	}
	err := server.ListenAndServeTLS(certPath, keyPath)
	log.Log.Error(err, "never should get here")
}
