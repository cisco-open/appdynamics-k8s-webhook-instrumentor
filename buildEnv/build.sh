#! /bin/bash

cd ..

docker build -f buildEnv/Dockerfile . -t chrlic/appd-webhook-instrumentor:v1.0.1

docker push chrlic/appd-webhook-instrumentor:v1.0.1

