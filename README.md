# AppDynamics K8S Webhook Instrumentor

This project provides K8S mutating webhook, which, by pre-set rules, auto-instruments pods at their creation time with AppDynamics or OpenTelemetry agents. 

## Supported Technologies

AppDynamics agents:
- Java
- .NET Core 
- Node.js 
- Apache (in progress)

AppDynamics agents in Hybrid mode (OpenTelemetry):
- Java

OpenTelemetry agents:
- Java
- Node.js (experimental)

Cisco Telescope:
- Java (in progress)
- Node.js (in progress)

## How to install?

User helm:
```
helm install --namespace=<namespace> <chart-name> . --values=<values-file>
```

to upgrade after values change:
- on OpenShift, you can use `helm upgrade`
- on Kubernetes, use `helm delete <chart-name>` `helm install ...` commands for the time being

## How to configure?

If using helm, modify values.yaml for helm chart parameters

See `values-sample.yaml` for inspiration - doc will be provided later.





