#! /bin/sh

export agentLogDir=$(echo "/opt/opentelemetry-webserver/agent/logs" | sed 's,/,\\/,g')
echo ${agentLogDir}
cat agent/opentelemetry-webserver-sdk/conf/appdynamics_sdk_log4cxx.xml.template | sed 's/__agent_log_dir__/'${agentLogDir}'/g' > agent/opentelemetry-webserver-sdk/conf/appdynamics_sdk_log4cxx.xml2