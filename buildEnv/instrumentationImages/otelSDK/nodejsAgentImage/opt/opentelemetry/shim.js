// tracing.js

'use strict'

const process = require('process');
const opentelemetry = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
// const { ConsoleSpanExporter } = require('@opentelemetry/sdk-trace-base');
const { Resource } = require('@opentelemetry/resources');
const { SemanticResourceAttributes } = require('@opentelemetry/semantic-conventions');
// const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-http');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
// const { JaegerExporter } = require('@opentelemetry/exporter-jaeger');
const { diag, DiagConsoleLogger, DiagLogLevel } = require('@opentelemetry/api');

// For troubleshooting, set the log level to DiagLogLevel.DEBUG
diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.DEBUG);

const otlpExporter = new OTLPTraceExporter({
  url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT
});
console.log(otlpExporter)

// configure the SDK to export telemetry data to the console
// enable all auto-instrumentations from the meta package
// const traceExporter = new ConsoleSpanExporter();
// console.log(traceExporter)
const sdk = new opentelemetry.NodeSDK({
  resource: new Resource({
    [SemanticResourceAttributes.SERVICE_NAME]: process.env.OTEL_SERVICE_NAME,
    [SemanticResourceAttributes.SERVICE_NAMESPACE]: process.env.OTEL_SERVICE_NAMESPACE,
  }),
  traceExporter: otlpExporter,
  serviceName: process.env.OTEL_SERVICE_NAME,
  instrumentations: [getNodeAutoInstrumentations()],
  autoDetectResources: true
});
console.log(sdk)

// initialize the SDK and register with the OpenTelemetry API
// this enables the API to record telemetry
sdk.start()
  .then(() => console.log('Tracing initialized'))
  .catch((error) => console.log('Error initializing tracing', error));

// gracefully shut down the SDK on process exit
process.on('SIGTERM', () => {
  sdk.shutdown()
    .then(() => console.log('Tracing terminated'))
    .catch((error) => console.log('Error terminating tracing', error))
    .finally(() => process.exit(0));
});