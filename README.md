# AppDynamics K8S Webhook Instrumentor

This project provides K8S mutating webhook, which, by pre-set rules, auto-instruments pods at their creation time with AppDynamics or OpenTelemetry agents. 

## Why?

### Why different autoinstrumentation for AppDynamics?

AppDynamics offers autoinstrumentation functionality via Cluster Agent and it's configuration. Cluster Agent uses a strategy of 
modifying Deployments, DeploymentConfigs (on OpenShift), and StatefulSets when those match pre-set criteria, when pod specifications are modified so, that agent for supported languages is automatically injected into application container. 
Kubernetes control plane then creates appropriate number of already instrumented pods. 

While this approach offers a few benefits, among all ability to reverse instrumentation changes in inprobable case of issues introduced by an agent, there are also a few drawbacks, especially when using modern popular CD tooling such as Argo CD. In some cases, Cluster Agent is unable to detect changes in deployment specification made by those tools, which results in application not being instrumented, in other cases, like when using Argo CD, Cluster Agent and CD deployment tool end up in endless cycle of back and forth modifications of deployment specification, resulting in undesired number of application restarts and inconsistent monitoring.

This project brings a method of agent injection directly to pod specification upon their instantiation wia mutating webhook, avoiding most of the corner cases mentioned above (the only significant being for example Argo CD deploying Pod directly and not via ReplicaSet  specification or Deployment/StatefulSet etc.). 

### Other use cases made possible

With OpenTelemetry support in AppDynamics SaaS, it can be assumed users will want the same level of ease of agent installation as in case of traditional AppDynamics agents. While there is an official OpenTelemetry autoistrumentation functionality available at https://github.com/open-telemetry/opentelemetry-operator for official OpenTelemetry agents, AppDynamics offers also hybrid agent functionality in AppDynamics agents where agents emit both native and OpenTelemetry telemetry data. 

This project supports both AppDynamics hybrid agents and OpenTelemetry agents in several ways including ability to create OpenTelemetry collectors with speciified custom configuration. 

## Supported Agents

| Language    | AppDynamics Native | AppDynamics Hybrid      | OpenTelemetry           | 
| ----------- | ------------------ | ----------------------- | ----------------------- | 
| Java        | :white_check_mark: | :white_check_mark:      | :white_check_mark:      | 
| .NET (Core) | :white_check_mark: | :thinking:              | :building_construction: | 
| Node.js     | :white_check_mark: | :building_construction: | :white_check_mark:      | 
| Apache      | :thinking:         | :x:                     | :microscope:            | 
| Nginx       | :x:                | :x:                     | :microscope:            | 
| Go          | :x:                | :x:                     | :thinking:              | 

|Icon                    |Support level           |
|------------------------|------------------------|
|:white_check_mark:      | Supported              |
|:x:                     | No plans at this time. |
|:microscope:            | Experimental           |
|:thinking:              | Under consideration / in planning   |
|:building_construction: | Under construction.    |


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

See `values-sample.yaml` for inspiration and this [Blog Post](<https://chrlic.github.io/appd-mwh-blog/>) to get started.

More examples and documentation is coming soon. 







