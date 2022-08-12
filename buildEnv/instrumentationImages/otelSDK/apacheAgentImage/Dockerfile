FROM alpine:latest

COPY ./build/agent/opentelemetry-webserver-sdk /opt/opentelemetry

RUN chmod a+w /opt/opentelemetry/logs

CMD ["cat", "Just delivering the Opentelemetry Apache/Nginx agent"]
