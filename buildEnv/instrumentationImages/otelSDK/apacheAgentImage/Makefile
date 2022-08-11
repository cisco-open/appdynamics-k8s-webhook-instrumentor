all: build images

build:
	./build.sh

images:
	docker build . -t chrlic/opentelemetry-apache-agent:latest

	docker push chrlic/opentelemetry-apache-agent:latest



