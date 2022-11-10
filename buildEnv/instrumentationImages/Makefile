all: otel-java-image otel-dotnet-image otel-nodejs-image telescope-java-image telescope-nodejs-image

otel-java-image: 
	$(MAKE) -C otelSDK/javaAgentImage images

otel-dotnet-image: 
	$(MAKE) -C otelSDK/dotnetAgentImage images

otel-nodejs-image: 
	$(MAKE) -C otelSDK/nodejsAgentImage images

telescope-java-image: 
	$(MAKE) -C telescopeSDK/javaAgentImage images

telescope-nodejs-image: 
	$(MAKE) -C telescopeSDK/nodejsAgentImage images

