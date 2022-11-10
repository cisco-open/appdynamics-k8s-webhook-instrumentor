# AppDynamics K8S Webhook Instrumentor

This project provides K8S webhook, which, by pre-set rules, auto-instruments Pods at their creation time with AppDynamics agent. 

## Supported Technologies

- Java
- .NET Core (in progress)
- Node.js (in progress)
- Apache (in progress)

## How to install?

Preferably, use the helm chart
```
helm -namespace <namespace> install <chart-name> .
```

or use
```
./deploy.sh
```
and change it per your needs

## How to configure?

If using helm, modify values.yaml for helm chart parameters

If using the script, deploy config map with configuration per example 




```
cd existing_repo
git remote add origin https://gitlab.com/Divis/appdynamics-k8s-webhook-instrumentor.git
git branch -M main
git push -uf origin main
```


