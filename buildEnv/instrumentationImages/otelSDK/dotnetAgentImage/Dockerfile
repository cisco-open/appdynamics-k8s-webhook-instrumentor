FROM alpine:latest

RUN mkdir /opt/opentelemetry
WORKDIR /opt/opentelemetry

RUN wget https://github.com/open-telemetry/opentelemetry-dotnet-instrumentation/releases/download/v0.2.0-beta.1/opentelemetry-dotnet-instrumentation-linux-glibc.zip && \
    unzip opentelemetry-dotnet-instrumentation-linux-glibc.zip && \
    wget https://www.nuget.org/api/v2/package/System.Diagnostics.DiagnosticSource/6.0.0 && \
    chmod a+rwx /opt/opentelemetry

CMD ["cat", "Just delivering the Opentelemetry .NET agent"]