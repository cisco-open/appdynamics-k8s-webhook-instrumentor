# /bin/bash


kubectl delete namespace webhook

kubectl delete mutatingwebhookconfiguration demo-webhook

kubectl delete clusterrole webhook-instrumentor

kubectl delete clusterrolebinding webhook-instrumentor

sleep 2

./deploy.sh

sleep 2

kubectl apply -f cm-test.yaml

kubectl -nwebhook get services
kubectl -nwebhook get pods
