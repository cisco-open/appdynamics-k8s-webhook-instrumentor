const { ciscoTracing } = require('@cisco-telescope/cisco-sdk-node');

const userOptions = {
  serviceName: process.env.OTEL_SERVICE_NAME,
  serviceNamespace: process.env.OTEL_SERVICE_NAMESPACE,
  exporters: [
    {
      type: 'otlp-http',
      collectorEndpoint: process.env.OTEL_EXPORTER_OTLP_ENDPOINT,
    },
  ],
};

ciscoTracing.init(userOptions); // init() is an asynchronous function. Consider calling it in 'async-await' format

/*
const userOptions = {
  serviceName: 'my-app-name',
  exporters: [
    {
      type: 'otlp-grpc',
      collectorEndpoint: 'grpc://localhost:4317',
      customHeaders: {
        'someheader-to-inject': 'header value',
      },
    },
  ],
};
*/