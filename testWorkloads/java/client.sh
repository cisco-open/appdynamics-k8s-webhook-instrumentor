#! /bin/bash

while true 
do
  curl http://localhost:8383/upstream/hello
  echo ""
  sleep 20
done