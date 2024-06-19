FROM busybox

ARG version

ADD https://github.com/signalfx/splunk-otel-java/releases/download/v$version/splunk-otel-javaagent.jar /opt/splunk/splunk-otel-javaagent.jar

RUN chmod -R go+r /opt/splunk/splunk-otel-javaagent.jar