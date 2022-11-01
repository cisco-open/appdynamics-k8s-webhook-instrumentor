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
	"context"
	"log"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

type OtelConfig struct {
	Trace             bool
	Endpoint          string
	SamplesPerMillion int64
	LogPayload        bool
	ServiceName       string
	ServiceNamespace  string
}

var otelConfig OtelConfig

// Initializes an OTLP exporter, and configures the corresponding trace and
// metric providers.
func initOtelTracing() func() {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String(otelConfig.ServiceName),
			semconv.ServiceNamespaceKey.String(otelConfig.ServiceNamespace),
			semconv.TelemetrySDKLanguageGo,
		),
	)
	handleErr(err, "failed to create resource")

	conn, err := grpc.DialContext(ctx, otelConfig.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()) /*, grpc.WithBlock()*/)
	handleErr(err, "failed to create gRPC connection to collector")

	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	handleErr(err, "failed to create trace exporter")

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		// sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(float64(otelConfig.SamplesPerMillion/1000000))),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func() {
		// Shutdown will flush any remaining spans and shut down the exporter.
		handleErr(tracerProvider.Shutdown(ctx), "failed to shutdown TracerProvider")
	}
}

func handleErr(err error, message string) {
	if err != nil {
		log.Printf("%s: %v", message, err)
	}
}

func getTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func otelAddAttrToSpan(ctx context.Context, key string, value string) {
	if !otelConfig.LogPayload {
		return
	}
	span := trace.SpanFromContext(ctx)
	// log.Printf("Tracing span %v\n", span.SpanContext())
	span.SetAttributes(attribute.String(key, value))
}

func otelHandler(handler http.Handler, spanName string) http.Handler {
	return otelhttp.NewHandler(handler, spanName)
}

func otelAddAttrs(ctx context.Context, requestBody, responseBody []byte) {
	otelAddAttrToSpan(ctx, "http.request.dump", string(requestBody))
	otelAddAttrToSpan(ctx, "http.response.dump", string(responseBody))
}
