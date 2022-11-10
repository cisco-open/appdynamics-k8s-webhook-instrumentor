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
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	admission "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	podResource = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
)

func applyAppdInstrumentation(req *admission.AdmissionRequest) ([]patchOperation, error) {
	// This handler should only get called on Pod objects as per the MutatingWebhookConfiguration in the YAML file.
	// However, if (for whatever reason) this gets invoked on an object of a different kind, issue a log message but
	// let the object request pass through otherwise.
	if req.Resource != podResource {
		log.Printf("expect resource to be %s", podResource)
		return nil, nil
	}

	// Parse the Pod object.
	raw := req.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &pod); err != nil {
		return nil, fmt.Errorf("could not deserialize pod object: %v", err)
	}

	// Check if we have configuration sucessfully
	if config.ControllerConfig == nil || config.InstrumentationConfig == nil {
		return nil, fmt.Errorf("instrumentor configuration not read from configmap")
	}

	log.Printf("Checking instrumentation for pod: %s", pod.Name)
	instrumentationRule := getInstrumentationRule(pod)

	if instrumentationRule == nil { // pod not eligible for AppDynamics instrumentation
		return []patchOperation{}, nil
	}

	log.Printf("Found instrumentation rule: %s", instrumentationRule.Name)

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
		log.Printf("Using otel tracing: %s\n", otelConfig)
		shutdown := initOtelTracing()
		defer shutdown()
	}

	go runConfigWatcher()

	// select {}
	// return

	certPath := filepath.Join(tlsDir, tlsCertFile)
	keyPath := filepath.Join(tlsDir, tlsKeyFile)

	tracer := getTracer("webhook-tracer")
	log.Printf("Otel Tracer: %v\n", tracer)

	mux := http.NewServeMux()

	mux.Handle("/mutate", otelHandler(admitFuncHandler(applyAppdInstrumentation), "/mutate"))
	server := &http.Server{
		// We listen on port 8443 such that we do not need root privileges or extra capabilities for this server.
		// The Service object will take care of mapping this port to the HTTPS port 443.
		Addr:    ":8443",
		Handler: mux,
	}
	log.Fatal(server.ListenAndServeTLS(certPath, keyPath))
}
