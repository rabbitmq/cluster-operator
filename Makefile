SHELL := bash
platform := $(shell uname | tr A-Z a-z)

.DEFAULT_GOAL = help
.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

ENVTEST_K8S_VERSION ?= 1.22.1
ARCHITECTURE = amd64
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
	setup-envtest --os $(platform) --arch $(ARCHITECTURE) --bin-dir $(LOCAL_TESTBIN) use $(ENVTEST_K8S_VERSION)

.PHONY: kubebuilder-assets
kubebuilder-assets: $(KUBEBUILDER_ASSETS)

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
manager: generate fmt vet vuln
	go mod download
	go build -o bin/manager main.go

deploy-manager:  ## Deploy manager
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/base | kubectl apply -f -

deploy-manager-dev:
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/overlays/dev | sed 's@((operator_docker_image))@"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"@' | kubectl apply -f -

deploy-sample: ## Deploy RabbitmqCluster defined in config/sample/base
	kustomize build config/samples/base | kubectl apply -f -

destroy: ## Cleanup all controller artefacts
	kustomize build config/crd/ | kubectl delete --ignore-not-found=true -f -
	kustomize build config/default/base/ | kubectl delete --ignore-not-found=true -f -
	kustomize build config/rbac/ | kubectl delete --ignore-not-found=true -f -
	kustomize build config/namespace/base/ | kubectl delete --ignore-not-found=true -f -

run: generate manifests fmt vet vuln install deploy-namespace-rbac just-run ## Run operator binary locally against the configured Kubernetes cluster in ~/.kube/config

just-run: ## Just runs 'go run main.go' without regenerating any manifests or deploying RBACs
	KUBECONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=$(K8S_OPERATOR_NAMESPACE) go run ./main.go -metrics-bind-address 127.0.0.1:9782 --zap-devel $(OPERATOR_ARGS)

delve: generate install deploy-namespace-rbac just-delve ## Deploys CRD, Namespace, RBACs and starts Delve debugger

just-delve: install-tools ## Just starts Delve debugger
	KUBECONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=$(K8S_OPERATOR_NAMESPACE) dlv debug

install: manifests ## Install CRDs into a cluster
	kubectl apply -f config/crd/bases

deploy-namespace-rbac:
	kustomize build config/namespace/base | kubectl apply -f -
	kustomize build config/rbac | kubectl apply -f -

deploy: manifests deploy-namespace-rbac deploy-manager ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config

deploy-dev: check-env-docker-credentials docker-build-dev manifests deploy-namespace-rbac docker-registry-secret deploy-manager-dev ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config, with local changes

deploy-kind: check-env-docker-repo git-commit-sha manifests deploy-namespace-rbac ## Load operator image and deploy operator into current KinD cluster
	docker buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	kind load docker-image $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/overlays/kind | sed 's@((operator_docker_image))@"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"@' | kubectl apply -f -

YTT_VERSION ?= v0.44.1
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

# Build the docker image
docker-build: check-env-docker-repo git-commit-sha
	docker buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):latest .

# Push the docker image
docker-push: check-env-docker-repo
	docker push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):latest

git-commit-sha:
ifeq ("", git diff --stat)
GIT_COMMIT=$(shell git rev-parse --short HEAD)
else
GIT_COMMIT=$(shell git rev-parse --short HEAD)-
endif

docker-build-dev: check-env-docker-repo  git-commit-sha
	docker buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	docker push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)

CMCTL = $(LOCAL_TESTBIN)/cmctl
$(CMCTL): | $(LOCAL_TMP) $(LOCAL_TESTBIN)
	curl -sSL -o $(LOCAL_TMP)/cmctl.tar.gz https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cmctl-$(platform)-$(shell go env GOARCH).tar.gz
	tar -C $(LOCAL_TMP) -xzf $(LOCAL_TMP)/cmctl.tar.gz
	mv $(LOCAL_TMP)/cmctl $(CMCTL)

CERT_MANAGER_VERSION ?= 1.9.2
.PHONY: cert-manager
cert-manager: | $(CMCTL) ## Setup cert-manager
	@echo "Installing Cert Manager"
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	$(CMCTL) check api --wait=5m

.PHONY: cert-manager-rm
cert-manager-rm:
	@echo "Deleting Cert Manager"
	kubectl delete -f https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml --ignore-not-found

kind-prepare: ## Prepare KIND to support LoadBalancer services
	# Note that created LoadBalancer services will have an unreachable external IP
	@kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/namespace.yaml
	@kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/metallb.yaml
	@kubectl apply -f config/metallb/config.yaml
	@kubectl create secret generic -n metallb-system memberlist --from-literal=secretkey="$(shell openssl rand -base64 128)"

kind-unprepare:  ## Remove KIND support for LoadBalancer services
	# remove MetalLB
	@kubectl delete -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/metallb.yaml
	@kubectl delete -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/namespace.yaml

system-tests: install-tools ## Run end-to-end tests against Kubernetes cluster defined in ~/.kube/config
	NAMESPACE="$(SYSTEM_TEST_NAMESPACE)" K8S_OPERATOR_NAMESPACE="$(K8S_OPERATOR_NAMESPACE)" ginkgo -nodes=3 --randomize-all -r system_tests/

kubectl-plugin-tests: ## Run kubectl-rabbitmq tests
	echo "running kubectl plugin tests"
	PATH=$(PWD)/bin:$$PATH ./bin/kubectl-rabbitmq.bats

tests: unit-tests integration-tests system-tests kubectl-plugin-tests

docker-registry-secret: check-env-docker-credentials
	echo "creating registry secret and patching default service account"
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) --docker-server='$(DOCKER_REGISTRY_SERVER)' --docker-username="$$DOCKER_REGISTRY_USERNAME" --docker-password="$$DOCKER_REGISTRY_PASSWORD" || true
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount rabbitmq-cluster-operator -p '{"imagePullSecrets": [{"name": "$(DOCKER_REGISTRY_SECRET)"}]}'

install-tools:
	go mod download
	grep _ tools/tools.go | awk -F '"' '{print $$2}' | xargs -t go install
	go install "golang.org/x/vuln/cmd/govulncheck@latest"

check-env-docker-repo: check-env-registry-server
ifndef OPERATOR_IMAGE
	$(error OPERATOR_IMAGE is undefined: path to the Operator image within the registry specified in DOCKER_REGISTRY_SERVER (e.g. rabbitmq/cluster-operator - without leading slash))
endif

check-env-docker-credentials: check-env-registry-server
ifndef DOCKER_REGISTRY_USERNAME
	$(error DOCKER_REGISTRY_USERNAME is undefined: Username for accessing the docker registry)
endif
ifndef DOCKER_REGISTRY_PASSWORD
	$(error DOCKER_REGISTRY_PASSWORD is undefined: Password for accessing the docker registry)
endif
ifndef DOCKER_REGISTRY_SECRET
	$(error DOCKER_REGISTRY_SECRET is undefined: Name of Kubernetes secret in which to store the Docker registry username and password)
endif

check-env-registry-server:
ifndef DOCKER_REGISTRY_SERVER
	$(error DOCKER_REGISTRY_SERVER is undefined: URL of docker registry containing the Operator image (e.g. registry.my-company.com))
endif
