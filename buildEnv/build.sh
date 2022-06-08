#! /bin/bash

cd ..

docker build -f buildEnv/Dockerfile . -t chrlic/webhook-test:latest

docker push chrlic/webhook-test:latest

