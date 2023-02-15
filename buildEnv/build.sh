#! /bin/bash

cd ..

docker build -f buildEnv/Dockerfile . -t chrlic/appd-webhook-instrumentor:v1.0.4-exp

docker push chrlic/appd-webhook-instrumentor:v1.0.4-exp

