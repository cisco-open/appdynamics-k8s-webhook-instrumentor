all: build images

build:
	./build.sh

images:
	docker build . -t chrlic/opentelemetry-webserver-agent:latest

	docker push chrlic/opentelemetry-webserver-agent:latest



