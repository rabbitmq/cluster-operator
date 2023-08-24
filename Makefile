SHELL := bash
platform := $(shell uname | tr A-Z a-z)
ARCHITECTURE := $(shell uname -m)

ifeq ($(ARCHITECTURE),x86_64)
	ARCHITECTURE=amd64
endif

.DEFAULT_GOAL = help
.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

### Helper functions
### https://stackoverflow.com/questions/10858261/how-to-abort-makefile-if-variable-not-set
check_defined = \
    $(strip $(foreach 1,$1, \
        $(call __check_defined,$1,$(strip $(value 2)))))
__check_defined = \
    $(if $(value $1),, \
        $(error Undefined $1$(if $2, ($2))$(if $(value @), \
                required by target '$@')))
###

# The latest 1.25 available for envtest
ENVTEST_K8S_VERSION ?= 1.25.0
LOCAL_TESTBIN = $(CURDIR)/testbin
$(LOCAL_TESTBIN):
	mkdir -p $@

LOCAL_TMP := $(CURDIR)/tmp
$(LOCAL_TMP):
	mkdir -p $@

K8S_OPERATOR_NAMESPACE ?= rabbitmq-system
SYSTEM_TEST_NAMESPACE ?= rabbitmq-system

# "Control plane binaries (etcd and kube-apiserver) are loaded by default from /usr/local/kubebuilder/bin.
# This can be overridden by setting the KUBEBUILDER_ASSETS environment variable"
# https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest
export KUBEBUILDER_ASSETS = $(LOCAL_TESTBIN)/k8s/$(ENVTEST_K8S_VERSION)-$(platform)-$(ARCHITECTURE)

$(KUBEBUILDER_ASSETS):
	setup-envtest -v info --os $(platform) --arch $(ARCHITECTURE) --bin-dir $(LOCAL_TESTBIN) use $(ENVTEST_K8S_VERSION)

.PHONY: kubebuilder-assets
kubebuilder-assets: $(KUBEBUILDER_ASSETS)

.PHONY: kubebuilder-assets-rm
kubebuilder-assets-rm:
	setup-envtest -v debug --os $(platform) --arch $(ARCHITECTURE) --bin-dir $(LOCAL_TESTBIN) cleanup

.PHONY: unit-tests
unit-tests: install-tools $(KUBEBUILDER_ASSETS) generate fmt vet vuln manifests ## Run unit tests
	ginkgo -r --randomize-all api/ internal/ pkg/

.PHONY: integration-tests
integration-tests: install-tools $(KUBEBUILDER_ASSETS) generate fmt vet vuln manifests ## Run integration tests
	ginkgo -r controllers/

manifests: install-tools ## Generate manifests e.g. CRD, RBAC etc.
	controller-gen crd rbac:roleName=operator-role paths="./api/...;./controllers/..." output:crd:artifacts:config=config/crd/bases
	./hack/remove-override-descriptions.sh
	./hack/add-notice-to-yaml.sh config/rbac/role.yaml
	./hack/add-notice-to-yaml.sh config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml

api-reference: install-tools ## Generate API reference documentation
	crd-ref-docs \
		--source-path ./api/v1beta1 \
		--config ./docs/api/autogen/config.yaml \
		--templates-dir ./docs/api/autogen/templates \
		--output-path ./docs/api/rabbitmq.com.ref.asciidoc \
		--max-depth 30

.PHONY: checks
checks::fmt ## Runs fmt + vet +govulncheck against the current code
checks::vet
checks::vuln

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Run govulncheck against code
vuln:
	govulncheck ./...

# Generate code & docs
generate: install-tools api-reference
	controller-gen object:headerFile=./hack/NOTICE.go.txt paths=./api/...
	controller-gen object:headerFile=./hack/NOTICE.go.txt paths=./internal/status/...

# Build manager binary
manager: generate checks
	go mod download
	go build -o bin/manager main.go

deploy-manager:  ## Deploy manager
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/base | kubectl apply -f -

deploy-manager-dev:
	@$(call check_defined, OPERATOR_IMAGE, path to the Operator image within the registry e.g. rabbitmq/cluster-operator)
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/overlays/dev | sed 's@((operator_docker_image))@"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"@' | kubectl apply -f -

deploy-sample: ## Deploy RabbitmqCluster defined in config/sample/base
	kustomize build config/samples/base | kubectl apply -f -

destroy: ## Cleanup all controller artefacts
	kustomize build config/crd/ | kubectl delete --ignore-not-found=true -f -
	kustomize build config/default/base/ | kubectl delete --ignore-not-found=true -f -
	kustomize build config/rbac/ | kubectl delete --ignore-not-found=true -f -
	kustomize build config/namespace/base/ | kubectl delete --ignore-not-found=true -f -

.PHONY: run
run::generate ## Run operator binary locally against the configured Kubernetes cluster in ~/.kube/config
run::manifests
run::checks
run::install
run::deploy-namespace-rbac
run::just-run

just-run: ## Just runs 'go run main.go' without regenerating any manifests or deploying RBACs
	KUBECONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=$(K8S_OPERATOR_NAMESPACE) go run ./main.go -metrics-bind-address 127.0.0.1:9782 --zap-devel $(OPERATOR_ARGS)

.PHONY: delve
delve::generate ## Deploys CRD, Namespace, RBACs and starts Delve debugger
delve::install
delve::deploy-namespace-rbac
delve::just-delve

just-delve: install-tools ## Just starts Delve debugger
	KUBECONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=$(K8S_OPERATOR_NAMESPACE) dlv debug

install: manifests ## Install CRDs into a cluster
	kubectl apply -f config/crd/bases

deploy-namespace-rbac:
	kustomize build config/namespace/base | kubectl apply -f -
	kustomize build config/rbac | kubectl apply -f -

.PHONY: deploy
deploy::manifests ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config
deploy::deploy-namespace-rbac
deploy::deploy-manager

.PHONY: deploy-dev
deploy-dev::docker-build-dev ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config, with local changes
deploy-dev::manifests
deploy-dev::deploy-namespace-rbac
deploy-dev::docker-registry-secret
deploy-dev::deploy-manager-dev

GIT_COMMIT := $(shell git rev-parse --short HEAD)
deploy-kind: manifests deploy-namespace-rbac ## Load operator image and deploy operator into current KinD cluster
	@$(call check_defined, OPERATOR_IMAGE, path to the Operator image within the registry e.g. rabbitmq/cluster-operator)
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	docker buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	kind load docker-image $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/overlays/kind | sed 's@((operator_docker_image))@"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"@' | kubectl apply -f -

YTT_VERSION ?= v0.45.3
YTT = $(LOCAL_TESTBIN)/ytt
$(YTT): | $(LOCAL_TESTBIN)
	mkdir -p $(LOCAL_TESTBIN)
	curl -sSL -o $(YTT) https://github.com/vmware-tanzu/carvel-ytt/releases/download/$(YTT_VERSION)/ytt-$(platform)-$(shell go env GOARCH)
	chmod +x $(YTT)

QUAY_IO_OPERATOR_IMAGE ?= quay.io/rabbitmqoperator/cluster-operator:latest
# Builds a single-file installation manifest to deploy the Operator
generate-installation-manifest: | $(YTT)
	mkdir -p releases
	kustomize build config/installation/ > releases/cluster-operator.yml
	$(YTT) -f releases/cluster-operator.yml -f config/ytt/overlay-manager-image.yaml --data-value operator_image=$(QUAY_IO_OPERATOR_IMAGE) > releases/cluster-operator-quay-io.yml

docker-build: ## Build the docker image with tag `latest`
	@$(call check_defined, OPERATOR_IMAGE, path to the Operator image within the registry e.g. rabbitmq/cluster-operator)
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	docker buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):latest .

docker-push: ## Push the docker image with tag `latest`
	@$(call check_defined, OPERATOR_IMAGE, path to the Operator image within the registry e.g. rabbitmq/cluster-operator)
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	docker push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):latest

docker-build-dev:
	@$(call check_defined, OPERATOR_IMAGE, path to the Operator image within the registry e.g. rabbitmq/cluster-operator)
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	docker buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	docker push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)

CMCTL = $(LOCAL_TESTBIN)/cmctl
$(CMCTL): | $(LOCAL_TMP) $(LOCAL_TESTBIN)
	curl -sSL -o $(LOCAL_TMP)/cmctl.tar.gz https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cmctl-$(platform)-$(shell go env GOARCH).tar.gz
	tar -C $(LOCAL_TMP) -xzf $(LOCAL_TMP)/cmctl.tar.gz
	mv $(LOCAL_TMP)/cmctl $(CMCTL)

CERT_MANAGER_VERSION ?= 1.9.2
.PHONY: cert-manager
cert-manager: | $(CMCTL) ## Setup cert-manager. Use CERT_MANAGER_VERSION to customise the version e.g. CERT_MANAGER_VERSION="1.9.2"
	@echo "Installing Cert Manager"
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	$(CMCTL) check api --wait=5m

.PHONY: cert-manager-rm
cert-manager-rm:
	@echo "Deleting Cert Manager"
	kubectl delete -f https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml --ignore-not-found

system-tests: install-tools ## Run end-to-end tests against Kubernetes cluster defined in ~/.kube/config
	NAMESPACE="$(SYSTEM_TEST_NAMESPACE)" K8S_OPERATOR_NAMESPACE="$(K8S_OPERATOR_NAMESPACE)" ginkgo -nodes=3 --randomize-all -r system_tests/

kubectl-plugin-tests: ## Run kubectl-rabbitmq tests
	@echo "running kubectl plugin tests"
	PATH=$(PWD)/bin:$$PATH ./bin/kubectl-rabbitmq.bats

.PHONY: tests
tests::unit-tests ## Runs all test suites: unit, integration, system and kubectl-plugin
tests::integration-tests
tests::system-tests
tests::kubectl-plugin-tests

docker-registry-secret:
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	@$(call check_defined, DOCKER_REGISTRY_USERNAME, Username for accessing the docker registry e.g. robot-123)
	@$(call check_defined, DOCKER_REGISTRY_PASSWORD, Password for accessing the docker registry e.g. password)
	@$(call check_defined, DOCKER_REGISTRY_SECRET, Name of Kubernetes secret in which to store the Docker registry username and password)
	@printf "creating registry secret and patching default service account"
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) --docker-server='$(DOCKER_REGISTRY_SERVER)' --docker-username="$$DOCKER_REGISTRY_USERNAME" --docker-password="$$DOCKER_REGISTRY_PASSWORD" || true
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount rabbitmq-cluster-operator -p '{"imagePullSecrets": [{"name": "$(DOCKER_REGISTRY_SECRET)"}]}'

.PHONY: install-tools
install-tools:
	grep _ tools/tools.go | awk -F '"' '{print $$2}' | xargs -t go install -mod=mod
