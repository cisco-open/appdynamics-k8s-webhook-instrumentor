# AppDynamics K8S Webhook Instrumentor

This project provides a auto-instrumentation tool primarily for AppDynamics agents running in Kubernetes cluster, but provides also easy instrumentation by OpenTelemetry agents and other supported agents. Under the hood, it uses K8S mutating webhook, which, by pre-set rules, auto-instruments pods at their creation time with AppDynamics or other supported agents. 

## Why?

### Why different auto-instrumentation for AppDynamics?

AppDynamics offers auto-instrumentation functionality via Cluster Agent and it's configuration. Cluster Agent uses a strategy of 
modifying Deployments, DeploymentConfigs (on OpenShift), and StatefulSets when those match pre-set criteria, when pod specifications are modified so, that agent for supported languages is automatically injected into application container. 
Kubernetes control plane then creates appropriate number of already instrumented pods. 

While this approach offers a few benefits, among all ability to reverse instrumentation changes in improbable case of issues introduced by an agent, there are also a few drawbacks, especially when using modern popular CD tooling such as Argo CD. In some cases, Cluster Agent is unable to detect changes in deployment specification made by those tools, which results in application not being instrumented, in other cases, like when using Argo CD, Cluster Agent and CD deployment tool end up in endless cycle of back and forth modifications of deployment specification, resulting in undesired number of application restarts and inconsistent monitoring.

This project brings a method of agent injection directly to pod specification upon their instantiation wia mutating webhook, avoiding most of the corner cases mentioned above (the only significant being for example Argo CD deploying Pod directly and not via ReplicaSet  specification or Deployment/StatefulSet etc.). 

### Other use cases made possible

With OpenTelemetry support in AppDynamics SaaS, it can be assumed users will want the same level of ease of agent installation as in case of traditional AppDynamics agents. While there is an official OpenTelemetry auto-instrumentation functionality available at https://github.com/open-telemetry/opentelemetry-operator for official OpenTelemetry agents, AppDynamics offers also hybrid agent functionality in AppDynamics agents where agents emit both native and OpenTelemetry telemetry data. 

This project supports both AppDynamics hybrid agents and OpenTelemetry agents in several ways including ability to create OpenTelemetry collectors with specified custom configuration. 

## Supported Agents

| Agent Type / Language | AppDynamics Native | AppDynamics Hybrid      | OpenTelemetry           | Splunk                  |
| --------------------- | ------------------ | ----------------------- | ----------------------- | ----------------------- |
| Java                  | :white_check_mark: | :white_check_mark:      | :white_check_mark:      | :building_construction: |
| .NET (Core)           | :white_check_mark: | :thinking:              | :white_check_mark:      | :x:                     |
| Node.js               | :white_check_mark: | :building_construction: | :white_check_mark:      | :building_construction: |
| Apache                | :thinking:         | :x:                     | :white_check_mark:      | :x:                     |
| Nginx                 | :x:                | :x:                     | :white_check_mark:      | :x:                     |
| Go                    | :x:                | :x:                     | :thinking:              | :x:                     |

|Icon                    |Support level           |
|------------------------|------------------------|
|:white_check_mark:      | Supported              |
|:x:                     | No plans at this time. |
|:microscope:            | Experimental           |
|:thinking:              | Under consideration / in planning   |
|:building_construction: | Under construction.    |


## How to install?

Using Helm:

1) Add Helm chart repository 
   
```
helm repo add mwh https://cisco-open.github.io/appdynamics-k8s-webhook-instrumentor/
```

2) Update Helm repository cache
   
```
helm repo update
```

3) Deploy chart with values

```
helm install --namespace=<namespace> <chart-name> mwh/webhook-instrumentor --values=<values-file>
```

(For `values.yaml` file, see **How to configure?** section)

to upgrade after values change:
- on OpenShift, you can use `helm upgrade`
- on Kubernetes, use `helm delete <chart-name>` `helm install ...` commands for the time being

## How to configure?

Before deploying via Helm chart, modify values.yaml for helm chart parameters

See `values-sample.yaml` in `webhook/helm` directory for inspiration and this [Blog Post](<https://chrlic.github.io/appd-mwh-blog/>) to get started.

The Helm chart always deploys the mutating webhook processes, set's up necessary security for the instrumentation to work. 

Instrumentation rules and OpenTelemetry collectors in use can be defined in tow ways:
- as part of `values.yaml` file for Helm chart
- via custom resource definitions (CRDs)
- by combination of both

### Using CRDs for Instrumentation rule definition

There are two custom resources definitions created by the Helm chart enabling to define application instrumentation rules:
- API `ext.appd.com/v1alpha1` resource `Instrumentation` - namespaced resource, which defines instrumentation rules
- API `ext.appd.com/v1alpha1` resource `ClusterInstrumentation` - cluster-wide resource, which defines instrumentation rules

Syntax and options for instrumentation rules are common across `values.yaml`, `Instrumentation`, and `ClusterInstrumentation` and the rule is applied by first match of the `matchRules` defined in individual rules. The order of evaluation is following:
- Rules defined by `Instrumentation` definitions in the same namespace, as the instrumented pod. Each rule can have `.spec.priority` defined (default = 1), higher priorities are evaluated first. Order of rules with the same priority is not deterministic.
- Rules defined by `ClusterInstrumentation` definitions. Those rules are shared for all pods and namespaces, unless the namespace is excluded. Each rule can have `.spec.priority` defined (default = 1), higher priorities are evaluated first. Order of rules with the same priority is not deterministic.
- Rules defined by the `values.yaml` file for Helm chart. Those rules are evaluated in the order they are present in the file. Unlike rules defined by CRDs, rules can use templates to simplify the definitions. 

Example:

~~~
apiVersion: ext.appd.com/v1alpha1
kind: Instrumentation
metadata:
  name: java-instrumentation
spec:
  name: java-instrumentation
  priority: 2
  matchRules:
    # must match both labels and their values.
    # values defined here are regexes!
    labels: 
    - language: java 
    - otel: appd
    podNameRegex: .*
  injectionRules:
    technology: java     # defines the agent type to use
    image: appdynamics/java-agent:latest
    javaEnvVar: _JAVA_OPTIONS
    openTelemetryCollector: test # enables OpenTelemetry and defines the collector to use
~~~

### Using CRDs for OpenTelemetry collector definition

When using OpenTelemetry, collector generally has to be deployed somewhere, usually on the same K8S cluster. This tool enables to provision 3 `.spec.mode` of collectors:
- `deployment` - collector running as `Deployment` with a `Service` published for application to send the signals to. This is managed by this tool.
- `sidecar`  - collector running as a sidecar in the same pod as application itself. This is injected into the pod by this tool
- `external` - collector running independently of this tool, not managed by this tool - it's expected you setup this collector another way yourselves. It can even run anywhere like on different cluster, VM, or in the cloud. 

When using resource `ext.appd.com/v1alpha1` - `OpenTelemetryCollector`, and `deployment` mode, this tool will spin up a collector in the same namespace where this resource definition is created. When you delete the `OpenTelemetryCollector` resource, collector will be also deleted. 

Example:

~~~
apiVersion: ext.appd.com/v1alpha1
kind: OpenTelemetryCollector
metadata:
  name: test
spec:
  replicas: 1
  image: otel/opentelemetry-collector-contrib:latest
  imagePullPolicy: Always
  mode: deployment
  config: |-
    receivers:
      otlp:
        protocols:
          grpc:
          http:
    processors:
      batch:
      resource:
        attributes:
        - key: appdynamics.controller.account
          action: upsert
          value: "<<APPD_ACCOUNT>>"
        - key: appdynamics.controller.host
          action: upsert     
          value: "<<APPD_CONTROLLER>>"
        - key: appdynamics.controller.port
          action: upsert
          value: <<APPD_CONTROLLER_PORT>>
    exporters:
      logging:
        loglevel: debug
      # This part says that the opentelemetry collector will send data to OTIS pipeline for AppDynamicas CSaaS.
      otlphttp:
        tls:
          insecure: true
        endpoint: "<<APPD_OTEL_SERVICE>>"
        headers: {"x-api-key": <<APPD_OTEL_AUTH_KEY>>}
    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [batch, resource]
          exporters: [logging, otlphttp]
      telemetry:
        logs:
          level: "debug"
~~~

More examples and documentation is coming soon. 

## DB Agent support

DB Agent can be provisioned with the instrumentor, too. Add following section to your values file, note there can be multiple DB agents provisioned:

```
dbAgents:
  # db agent name MUST be DNS friendly - only lowercase, numbers and "-"
  md-db-agent-k8s:
    image:
      image: docker.io/appdynamics/db-agent:latest
      imagePullPolicy: Always
```

DB Agent will automatically register to the AppDynamics controller specified for instrumentation in the values file.






