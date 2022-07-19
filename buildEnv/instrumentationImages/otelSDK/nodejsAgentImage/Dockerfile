FROM node:lts-slim

RUN mkdir /opt/opentelemetry
COPY opt/opentelemetry /opt/opentelemetry
WORKDIR /opt/opentelemetry
RUN npm install \
  @opentelemetry/sdk-node \
  @opentelemetry/api \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-http \
  @opentelemetry/exporter-trace-otlp-grpc \
  @opentelemetry/semantic-conventions \
  process

CMD ["cat", "Just delivering the Opentelemetry Node.js agent"]