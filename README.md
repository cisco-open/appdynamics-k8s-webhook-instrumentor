# AppDynamics K8S Webhook Instrumentor

This project provides K8S mutating webhook, which, by pre-set rules, auto-instruments pods at their creation time with AppDynamics or OpenTelemetry agents. 

## Supported Agents

| Language    | AppDynamics Native | AppDynamics Hybrid      | OpenTelemetry           | Cisco Telescope    |
| Framework   |                    |                         |                         |                    |
| ----------- | ------------------ | ----------------------- | ----------------------- | ------------------ |
| Java        | :white_check_mark: | :white_check_mark:      | :white_check_mark:      | :white_check_mark: |
| .NET (Core) | :white_check_mark: | :x:                     | :building_construction: | :thinking:         |
| Node.js     | :white_check_mark: | :building_construction: | :white_check_mark:      | :white_check_mark: *) |
| Apache      | :thinking:         | :x:                     | :test_tube:             | :x:                |
| Nginx       | :x:                | :x:                     | :test_tube:             | :x:                |
| Go          | :x:                | :x:                     | :thinking:              | :x:                |

*) Does not work OOB with AppDynamics cSaaS contoller - service namespace resource attribute is not propagated. If needed, it can be fixed in Otel Collector.

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





