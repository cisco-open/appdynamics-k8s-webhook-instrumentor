FROM alpine:latest

RUN mkdir /opt/telescope
WORKDIR /opt/telescope

RUN wget https://github.com/cisco-open/otel-java/releases/latest/download/cisco-telescope-javaagent.jar
# RUN wget https://github.com/cisco-open/otel-java/releases/download/v0.0.2/cisco-telescope-javaagent.jar

CMD ["cat", "Just delivering the Opentelemetry Java agent"]