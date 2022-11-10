const { ciscoTracing } = require('@cisco-telescope/cisco-sdk-node');

const userOptions = {
  serviceName: process.env.TELESCOPE_SERVICE_NAME,
  ciscoToken: process.env.TELESCOPE_TOKEN,
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