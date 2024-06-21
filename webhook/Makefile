LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.14.0
CRD_OPTIONS ?= "crd:generateEmbeddedObjectMeta=true"

IMAGE_OWNER ?= chrlic
IMAGE_NAME ?= appd-webhook-instrumentor-ng
IMAGE_VERSION ?= 2.0.0

HELM_DEPLOYMENT_NAME ?= mwh
HELM_DEPLOYMENT_NAMESPACE ?= mwh
HELM_VALUES_FILE ?= values.yaml

TEST_SECRETS = ../e2e-tests/secrets.txt
TEST_HELM_VALUES = ../e2e-tests/helmDefaultValues.yaml
TEST_HELM_VALUES_TEMP = ../e2e-tests/helmDefaultValuesTemp.yaml

.PHONY: all
all: manifests api-docs

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: manifests
manifests: controller-gen
	cd pkg && $(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=appd-aac webhook paths="./..." output:crd:artifacts:config=../config/crd/bases
	cd pkg && $(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=appd-aac output:rbac:dir=../helm/instrumentor/templates paths="./..." output:crd:artifacts:config=../helm/instrumentor/templates object:headerFile=../license.txt

.PHONY: images
images:
	docker build . -t $(IMAGE_OWNER)/$(IMAGE_NAME):$(IMAGE_VERSION)
	docker push $(IMAGE_OWNER)/$(IMAGE_NAME):$(IMAGE_VERSION)

CRDOCBIN = $(shell pwd)/bin
CRDOC = $(shell pwd)/bin/crdoc
.PHONY: crdoc
crdoc: ## Download crdoc locally if necessary.
	GOBIN=$(CRDOCBIN) go install fybrik.io/crdoc@latest

HELM = "helm"
.PHONY: api-docs
api-docs: crdoc
	@{ \
	set -e ;\
	TMP_DIR=$$(mktemp -d) ; \
	$(HELM) template test helm/. >  $$TMP_DIR/crd-output.yaml;\
	$(CRDOC) --resources $$TMP_DIR/crd-output.yaml --output docs/api.md ;\
	}

.PHONY: install
install:
	cd helm && helm install --namespace $(HELM_DEPLOYMENT_NAMESPACE) $(HELM_DEPLOYMENT_NAME) . --values=$(HELM_VALUES_FILE)

.PHONY: install-test
install-test:
	cd helm && helm install --namespace $(HELM_DEPLOYMENT_NAMESPACE) $(HELM_DEPLOYMENT_NAME) . --values=$(TEST_HELM_VALUES_TEMP)

.PHONY: remove
remove:
	-cd helm && helm delete --namespace $(HELM_DEPLOYMENT_NAMESPACE) $(HELM_DEPLOYMENT_NAME)

.PHONY: test
test:
	cd pkg && TEST_SECRETS=$(TEST_SECRETS) TEST_HELM_VALUES=$(TEST_HELM_VALUES) TEST_HELM_VALUES_TEMP=$(TEST_HELM_VALUES_TEMP) go test *.go -v

.PHONY: test-cleanup
test-cleanup:
	-helm uninstall --namespace mwh mwh 
	-kubectl delete -f ./helm/instrumentor/templates/ext.appd.com_clusterinstrumentations.yaml
	-kubectl delete -f ./helm/instrumentor/templates/ext.appd.com_instrumentations.yaml       
	-kubectl delete -f ./helm/instrumentor/templates/ext.appd.com_opentelemetrycollectors.yaml
	-kubectl delete clusterrole webhook-instrumentor
	-kubectl delete clusterrole appd-aac
	-kubectl delete clusterrolebinding webhook-instrumentor
	-kubectl delete MutatingWebhookConfiguration webhook-appd

.PHONY: build-run
build-run: remove images install