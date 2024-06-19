IMAGE_REPO=chrlic

.PHONY: images
images: splunk-java-image

.PHONY: splunk-java-image
splunk-java-image:
	$(eval JAVA_VERSION := $(shell cat splunk/java/version.txt))
	cd splunk/java && docker build . -t $(IMAGE_REPO)/splunk-java-agent:$(JAVA_VERSION) --build-arg="version=$(JAVA_VERSION)"
	docker push $(IMAGE_REPO)/splunk-java-agent:$(JAVA_VERSION)