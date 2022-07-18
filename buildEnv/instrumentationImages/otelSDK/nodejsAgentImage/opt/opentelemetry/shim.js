const traceProvider = new NodeTracerProvider({
  resource: Resource(),
});
const collectorOptions = {
  url: 'https://production.cisco-udp.com/trace-collector', ////process.env.OTEL_EXPORTER_OTLP_ENDPOINT
  headers: {
    authorization: 'Bearer <Your Telescope Token>', //process.env.TELESCOPE_TOKEN - this not needed
  },
};
const httpExporter = new HTTPTraceExporter(collectorOptions);
traceProvider.addSpanProcessor(new BatchSpanProcessor(httpExporter));

/*
'use strict'

const process = require('process');
const opentelemetry = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { ConsoleSpanExporter } = require('@opentelemetry/sdk-trace-base');
const { Resource } = require('@opentelemetry/resources');
const { SemanticResourceAttributes } = require('@opentelemetry/semantic-conventions');

// configure the SDK to export telemetry data to the console
// enable all auto-instrumentations from the meta package
const traceExporter = new ConsoleSpanExporter();
const sdk = new opentelemetry.NodeSDK({
  resource: new Resource({
    [SemanticResourceAttributes.SERVICE_NAME]: 'my-service',
  }),
  traceExporter,
  instrumentations: [getNodeAutoInstrumentations()]
});

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








const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-http');

const exporter = new OTLPTraceExporter({
  // optional - url default value is http://localhost:4318/v1/traces
  url: '<your-collector-endpoint>/v1/traces',

  // optional - collection of custom headers to be sent with each request, empty by default
  headers: {}, 
});
*/