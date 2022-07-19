# AppDynamics K8S Webhook Instrumentor

This project provides K8S mutating webhook, which, by pre-set rules, auto-instruments pods at their creation time with AppDynamics or OpenTelemetry agents. 

## Supported Agents

| Language | AppDynamics Native | AppDynamics Hybrid | OpenTelemetry | Cisco Telescope |
| -------- | ------------------ | ------------------ | ------------- | --------------- |
| Java     | yes                | yes                | yes           | yes (experimental) |
| .NET (Core) | yes             | N/A                | N/A           | N/A             |
| Node.js  | yes                | yes                | yes           | in planning     |
| Apache   | in planning        | N/A                | in planning   | N/A             |
| Go       | no                 | no                 | in planning   | N/A.            |


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





