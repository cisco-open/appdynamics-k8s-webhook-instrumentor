#! /bin/bash

escapeForSedReplacement() {
    local __resultVarName str __result
    __resultVarName="$1"
    str="$2"
    __result=$(echo "$str" | sed 's/[/&]/\\&/g')
    eval $__resultVarName=\'$__result\'
}

rm -rf opentelemetry-cpp-contrib
git clone https://github.com/open-telemetry/opentelemetry-cpp-contrib
cd  opentelemetry-cpp-contrib/instrumentation/otel-webserver-module

docker build . -f docker/centos7/Dockerfile -t chrlic-apache-otel-build

docker run -idt --name chrlic-apache-otel-build chrlic-apache-otel-build /bin/sh -c "sleep 100" &
sleep 10

docker cp chrlic-apache-otel-build:/otel-webserver-module/build/opentelemetry-webserver-sdk-x64-linux.tgz ../../../build/

docker kill chrlic-apache-otel-build
docker rm chrlic-apache-otel-build

cd ../../../build

tar -xvf opentelemetry-webserver-sdk-x64-linux.tgz -C agent
cd agent/opentelemetry-webserver-sdk

agentLogDir="/opt/opentelemetry-apache/agent/logs"
escapeForSedReplacement agentLogDir "${agentLogDir}"
cat conf/appdynamics_sdk_log4cxx.xml.template | sed "s/__agent_log_dir__/${agentLogDir}/g"  > conf/appdynamics_sdk_log4cxx.xml
# ./install.sh


