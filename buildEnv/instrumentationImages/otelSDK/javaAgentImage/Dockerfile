FROM alpine:latest

RUN mkdir /opt/opentelemetry
WORKDIR /opt/opentelemetry

RUN wget https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar && \
    chmod a+rwx /opt/opentelemetry

CMD ["cat", "Just delivering the Opentelemetry Java agent"]