#! /bin/bash

cd ..

docker build -f buildEnv/Dockerfile . -t chrlic/appd-webhook-instrumentor:latest

docker push chrlic/appd-webhook-instrumentor:latest

