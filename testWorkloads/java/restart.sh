#! /bin/bash

kubectl -nr delete -f downstream/d-downstream.yaml
kubectl -nr delete -f upstream/d-upstream.yaml

kubectl -nr apply -f downstream/d-downstream.yaml
kubectl -nr apply -f upstream/d-upstream.yaml

while [ true ]
do
  kubectl -nr get pods
  sleep 10
done