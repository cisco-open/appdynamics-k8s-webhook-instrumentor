FROM alpine:latest

RUN mkdir /opt/telescope
WORKDIR /opt/telescope

RUN wget https://github.com/cisco-open/otel-java/releases/latest/download/cisco-telescope-javaagent.jar

CMD ["cat", "Just delivering the Telescope Java agent"]