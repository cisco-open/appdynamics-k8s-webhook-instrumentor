#! /bin/bash

cd ..

docker build -f buildEnv/Dockerfile . -t chrlic/appd-webhook-instrumentor:v1.0.6-crd

docker push chrlic/appd-webhook-instrumentor:v1.0.6-crd

