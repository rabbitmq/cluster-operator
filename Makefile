SHELL := bash
platform := $(shell uname | tr A-Z a-z)
ARCHITECTURE := $(shell uname -m)

ifeq ($(ARCHITECTURE),x86_64)
	ARCHITECTURE=amd64
endif

ifeq ($(ARCHITECTURE),aarch64)
	ARCHITECTURE=arm64
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

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

LOCAL_TMP := $(CURDIR)/tmp
$(LOCAL_TMP):
	mkdir -p -v $(@)

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= $(LOCALBIN)/kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GINKGO_CLI ?= $(LOCALBIN)/ginkgo
GOVULNCHECK ?= $(LOCALBIN)/govulncheck
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs
YJ ?= $(LOCALBIN)/yj
YTT ?= $(LOCALBIN)/ytt
CMCTL ?= $(LOCALBIN)/cmctl

## Tool Versions
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.21.0
GOVULNCHECK_VERSION ?= v1.5.0
CRD_REF_DOCS_VERSION ?= v0.3.0
YJ_VERSION ?= v5.1.0
YTT_VERSION ?= v0.55.1
CMCTL_VERSION ?= v2.5.0
KIND_VERSION ?= v0.32.0
CERT_MANAGER_VERSION ?= v1.15.1

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

# Determine GINKGO_VERSION from the go.mod file to keep CLI and code in sync
GINKGO_VERSION := $(shell v='$(call gomodver,github.com/onsi/ginkgo/v2)'; \
  [ -n "$$v" ] || { echo "Could not determine GINKGO_VERSION" >&2; exit 1; }; \
  printf '%s\n' "$$v")

# ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION := $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

# ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION := $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

LOCAL_TESTBIN = $(CURDIR)/testbin

##@ Tools

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local testbin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCAL_TESTBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: ginkgo-cli
ginkgo-cli: $(GINKGO_CLI) ## Download ginkgo CLI locally if necessary.
$(GINKGO_CLI): $(LOCALBIN)
	$(call go-install-tool,$(GINKGO_CLI),github.com/onsi/ginkgo/v2/ginkgo,$(GINKGO_VERSION))

.PHONY: govulncheck
govulncheck: $(GOVULNCHECK) ## Download govulncheck locally if necessary.
$(GOVULNCHECK): $(LOCALBIN)
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck,$(GOVULNCHECK_VERSION))

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	$(call go-install-tool,$(CRD_REF_DOCS),github.com/elastic/crd-ref-docs,$(CRD_REF_DOCS_VERSION))

.PHONY: yj
yj: $(YJ) ## Download yj (YAML/JSON converter) locally if necessary.
$(YJ): $(LOCALBIN)
	$(call go-install-tool,$(YJ),github.com/sclevine/yj/v5,$(YJ_VERSION))

# https://github.com/carvel-dev/ytt/releases
.PHONY: ytt
ytt: $(YTT) ## Download ytt locally if necessary.
$(YTT): $(LOCALBIN)
	@[ -f "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)" ] && [ "$$(readlink -- "$(YTT)" 2>/dev/null)" = "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)" ] || { \
		printf "Downloading and installing Carvel YTT\n"; \
		curl -sSL -o "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)" https://github.com/carvel-dev/ytt/releases/download/$(YTT_VERSION)/ytt-$(platform)-$(ARCHITECTURE); \
		chmod +x "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)"; \
		printf "Carvel YTT $(YTT_VERSION) installed locally\n"; \
	}; \
	ln -sf "$$(realpath "$(YTT)-$(YTT_VERSION)-$(platform)-$(ARCHITECTURE)")" "$(YTT)"

# https://github.com/cert-manager/cmctl/releases
.PHONY: cmctl
cmctl: $(CMCTL) ## Download cmctl locally if necessary.
$(CMCTL): $(LOCALBIN) $(LOCAL_TMP)
	curl -sSL -o $(LOCAL_TMP)/cmctl.tar.gz https://github.com/cert-manager/cmctl/releases/download/$(CMCTL_VERSION)/cmctl_$(platform)_$(ARCHITECTURE).tar.gz
	tar -C $(LOCAL_TMP) -xzf $(LOCAL_TMP)/cmctl.tar.gz
	mv $(LOCAL_TMP)/cmctl $(CMCTL)

# https://github.com/kubernetes-sigs/kind/releases
.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	@[ -f "$(KIND)-$(KIND_VERSION)-$(platform)-$(ARCHITECTURE)" ] && [ "$$(readlink -- "$(KIND)" 2>/dev/null)" = "$(KIND)-$(KIND_VERSION)-$(platform)-$(ARCHITECTURE)" ] || { \
		printf "Downloading and installing kind\n"; \
		curl -sSL -o "$(KIND)-$(KIND_VERSION)-$(platform)-$(ARCHITECTURE)" https://github.com/kubernetes-sigs/kind/releases/download/$(KIND_VERSION)/kind-$(platform)-$(ARCHITECTURE); \
		chmod +x "$(KIND)-$(KIND_VERSION)-$(platform)-$(ARCHITECTURE)"; \
		printf "kind $(KIND_VERSION) installed locally\n"; \
	}; \
	ln -sf "$$(realpath "$(KIND)-$(KIND_VERSION)-$(platform)-$(ARCHITECTURE)")" "$(KIND)"

.PHONY: install-tools
install-tools: kustomize controller-gen envtest ginkgo-cli govulncheck crd-ref-docs yj ytt kind ## Install all tooling required to configure and build this repo
	@echo "All tools installed successfully"

##@ Testing

K8S_OPERATOR_NAMESPACE ?= rabbitmq-system
SYSTEM_TEST_NAMESPACE ?= cluster-operator-system-tests
RABBITMQ_SERVICE_TYPE ?= NodePort

# "Control plane binaries (etcd and kube-apiserver) are loaded by default from /usr/local/kubebuilder/bin.
# This can be overridden by setting the KUBEBUILDER_ASSETS environment variable"
# https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest
# Note: setup-envtest returns the full path including patch version (e.g., 1.35.0), so we capture it dynamically
KUBEBUILDER_ASSETS_PATH = $(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCAL_TESTBIN) -p path 2>/dev/null || echo "$(LOCAL_TESTBIN)/k8s/$(ENVTEST_K8S_VERSION)-$(platform)-$(ARCHITECTURE)")
export KUBEBUILDER_ASSETS = $(KUBEBUILDER_ASSETS_PATH)

.PHONY: kubebuilder-assets
kubebuilder-assets: $(ENVTEST) ## Download and set up kubebuilder test assets
	@echo "Setting up kubebuilder assets..."
	@mkdir -p $(LOCAL_TESTBIN)
	@path="$$("$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCAL_TESTBIN) -p path)"; \
	if [ -n "$$path" ] && [ -d "$$path" ]; then \
		chmod -R +w "$$path" 2>/dev/null || true; \
		echo "Kubebuilder assets ready at: $$path"; \
		echo "export KUBEBUILDER_ASSETS=$$path"; \
	else \
		echo "Error: Failed to set up kubebuilder assets" >&2; \
		exit 1; \
	fi

.PHONY: clean-testbin
clean-testbin: ## Clean testbin directory (fixes permission issues)
	@echo "Cleaning testbin directory..."
	@if [ -d "$(LOCAL_TESTBIN)" ]; then \
		chmod -R +w $(LOCAL_TESTBIN) 2>/dev/null || true; \
		rm -rf $(LOCAL_TESTBIN); \
		echo "testbin directory cleaned"; \
	else \
		echo "testbin directory does not exist"; \
	fi


.PHONY: unit-tests
unit-tests::install-tools ## Run unit tests
unit-tests::controller-gen
unit-tests::kubebuilder-assets
unit-tests::generate
unit-tests::fmt
unit-tests::vet
unit-tests::manifests
unit-tests::just-unit-tests

GINKGO_PROCS ?= 4

.PHONY: just-unit-tests
just-unit-tests: ## Run just unit tests without regenerating code
	$(GINKGO_CLI) -r -p --randomize-all --fail-on-pending --procs=$(GINKGO_PROCS) --label-filter="!integration" $(GINKGO_EXTRA) api/ internal/ pkg/

.PHONY: integration-tests
integration-tests::install-tools ## Run integration tests
integration-tests::controller-gen
integration-tests::kubebuilder-assets
integration-tests::generate
integration-tests::fmt
integration-tests::vet
integration-tests::manifests
integration-tests::just-integration-tests

.PHONY: just-integration-tests
just-integration-tests: kubebuilder-assets ## Run just integration tests without regenerating code
	$(GINKGO_CLI) -r -p --fail-on-pending --randomize-all --procs=$(GINKGO_PROCS) --label-filter="integration" $(GINKGO_EXTRA) internal/controller/

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate manifests e.g. CRD, RBAC etc.
	"$(CONTROLLER_GEN)" crd rbac:roleName=rabbitmq-cluster-operator-role paths="./api/...;./internal/controller/..." output:crd:artifacts:config=config/crd/bases
	"$(CONTROLLER_GEN)" webhook paths="./internal/webhook/..." output:webhook:artifacts:config=config/webhook
	./hack/remove-override-descriptions.sh
	./hack/add-notice-to-yaml.sh config/rbac/role.yaml
	./hack/add-notice-to-yaml.sh config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml
	./hack/add-notice-to-yaml.sh config/webhook/manifests.yaml

.PHONY: api-reference
api-reference: crd-ref-docs ## Generate API reference documentation
	"$(CRD_REF_DOCS)" \
		--source-path ./api/v1beta1 \
		--config ./docs/api/autogen/config.yaml \
		--templates-dir ./docs/api/autogen/templates \
		--output-path ./docs/api/rabbitmq.com.ref.asciidoc \
		--max-depth 30

.PHONY: checks
checks::fmt ## Runs fmt + vet + govulncheck against the current code
checks::vet
checks::vuln

# Run go fmt against code
.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

# Run govulncheck against code
.PHONY: vuln
vuln: govulncheck ## Run govulncheck against code
	"$(GOVULNCHECK)" ./...

# Generate code & docs
.PHONY: generate
generate: controller-gen api-reference ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations
	"$(CONTROLLER_GEN)" object:headerFile=./hack/NOTICE.go.txt paths=./api/...
	"$(CONTROLLER_GEN)" object:headerFile=./hack/NOTICE.go.txt paths=./internal/status/...

# Build manager binary
manager: generate checks
	go build -o bin/manager ./cmd

deploy-manager: kustomize ytt ## Deploy manager
	"$(KUSTOMIZE)" build config/default | $(KUBECTL) apply -f -

deploy-manager-dev: kustomize ytt
	@$(call check_defined, OPERATOR_IMAGE, path to the Operator image within the registry e.g. rabbitmq/cluster-operator)
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	"$(KUSTOMIZE)" build config/default \
	| "$(YTT)" -f - -f config/ytt/overlay-manager-image.yaml \
		--data-value operator_image="$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)" \
	| $(KUBECTL) apply -f -

deploy-sample: kustomize ## Deploy RabbitmqCluster defined in config/sample
	"$(KUSTOMIZE)" build config/samples | $(KUBECTL) apply -f -

destroy: kustomize ## Cleanup all controller artefacts
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=true -f -
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=true -f -

.PHONY: run
run::generate ## Run operator binary locally against the configured Kubernetes cluster in ~/.kube/config
run::manifests
run::checks
run::install
run::deploy-namespace-rbac
run::just-run

just-run: ## Just runs 'go run ./cmd' without regenerating any manifests or deploying RBACs
	KUBECONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=$(K8S_OPERATOR_NAMESPACE) ENABLE_DEBUG_PPROF=true go run ./cmd -metrics-bind-address 127.0.0.1:9782 --zap-devel $(OPERATOR_ARGS)

install: manifests ## Install CRDs into a cluster
	$(KUBECTL) apply -f config/crd/bases

deploy-namespace-rbac: kustomize
	"$(KUSTOMIZE)" build config/namespace/base | $(KUBECTL) apply -f -
	"$(KUSTOMIZE)" build config/rbac | $(KUBECTL) apply -f -

# IMG is the image to deploy when using `make deploy`
IMG ?= ghcr.io/rabbitmq/cluster-operator:latest

.PHONY: deploy
deploy: manifests kustomize ytt ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config
	"$(KUSTOMIZE)" build config/default | \
		"$(YTT)" -f - -f config/ytt/overlay-manager-image.yaml --data-value operator_image=$(IMG) | \
		$(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config
	"$(KUSTOMIZE)" build config/default | $(KUBECTL) delete --ignore-not-found=true -f -

.PHONY: deploy-secure-metrics
deploy-secure-metrics: manifests kustomize ytt ## Deploy operator with HTTPS metrics (requires cert-manager)
	"$(KUSTOMIZE)" build config/overlays/metrics-https | \
		"$(YTT)" -f - -f config/ytt/overlay-manager-image.yaml --data-value operator_image=$(IMG) | \
		$(KUBECTL) apply -f -

.PHONY: undeploy-secure-metrics
undeploy-secure-metrics: kustomize ## Undeploy controller with secure metrics
	"$(KUSTOMIZE)" build config/overlays/metrics-https | $(KUBECTL) delete --ignore-not-found=true -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=true -f -

.PHONY: deploy-dev
deploy-dev::docker-build-dev ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config, with local changes
deploy-dev::manifests
deploy-dev::install
deploy-dev::deploy-namespace-rbac
deploy-dev::docker-registry-secret
deploy-dev::deploy-manager-dev

CONTAINER ?= docker

GIT_COMMIT := $(shell git rev-parse --short HEAD)
deploy-kind: manifests kustomize ytt ## Load operator image and deploy operator into current KinD cluster
	@$(call check_defined, OPERATOR_IMAGE, path to the Operator image within the registry e.g. rabbitmq/cluster-operator)
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	$(CONTAINER) buildx build --build-arg=DOCKER_REGISTRY=$(DOCKER_REGISTRY_SERVER) --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	$(KIND) load docker-image $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)
	$(KUSTOMIZE) build config/namespace/base | $(KUBECTL) apply -f -
	$(KUSTOMIZE) build config/default | \
		$(YTT) -f - -f config/ytt/overlay-manager-image.yaml \
			--data-value operator_image="$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)" \
			-f config/ytt/never_pull.yaml | \
		$(KUBECTL) apply -f -

QUAY_IO_OPERATOR_IMAGE ?= quay.io/rabbitmqoperator/cluster-operator:latest
GHCR_IO_OPERATOR_IMAGE ?= ghcr.io/rabbitmq/cluster-operator:latest
# Builds a single-file installation manifest to deploy the Operator
generate-installation-manifest: kustomize ytt ## Generate installation manifests
	mkdir -p releases
	$(KUSTOMIZE) build config/installation/ > releases/cluster-operator_base.yml
	$(YTT) -f releases/cluster-operator_base.yml -f config/ytt/overlay-manager-image.yaml --data-value operator_image=$(GHCR_IO_OPERATOR_IMAGE) > releases/cluster-operator.yml
	$(YTT) -f releases/cluster-operator_base.yml -f config/ytt/overlay-manager-image.yaml --data-value operator_image=$(QUAY_IO_OPERATOR_IMAGE) > releases/cluster-operator-quay-io.yml
	$(YTT) -f releases/cluster-operator_base.yml -f config/ytt/overlay-manager-image.yaml --data-value operator_image=$(GHCR_IO_OPERATOR_IMAGE) > releases/cluster-operator-ghcr-io.yml

docker-build: ## Build docker image with the manager. Use IMG to set image name.
	$(CONTAINER) buildx build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(IMG) .

docker-push: ## Push the docker image with tag `latest`
	@$(call check_defined, OPERATOR_IMAGE, path to the Operator image within the registry e.g. rabbitmq/cluster-operator)
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	$(CONTAINER) push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):latest

docker-build-dev:
	@$(call check_defined, OPERATOR_IMAGE, path to the Operator image within the registry e.g. rabbitmq/cluster-operator)
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	$(CONTAINER) buildx build --build-arg=DOCKER_REGISTRY=$(DOCKER_REGISTRY_SERVER) --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	$(CONTAINER) push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)

##@ Cert Manager

.PHONY: cert-manager
.PHONY: wait-for-webhook
wait-for-webhook: ## Wait for webhook CA bundles to be injected and the webhook service to be ready
	@echo "Waiting for the mutating webhook CA bundle to be injected..."
	@timeout 120s bash -c 'until $(KUBECTL) get mutatingwebhookconfigurations.admissionregistration.k8s.io cluster-operator-mutating-webhook-configuration -o jsonpath="{.webhooks[0].clientConfig.caBundle}" 2>/dev/null | grep -q "[a-zA-Z0-9]"; do sleep 2; done' || (echo "Timeout waiting for mutating webhook CA bundle injection" && exit 1)
	@echo "Waiting for the validating webhook CA bundle to be injected..."
	@timeout 120s bash -c 'until $(KUBECTL) get validatingwebhookconfigurations.admissionregistration.k8s.io cluster-operator-validating-webhook-configuration -o jsonpath="{.webhooks[0].clientConfig.caBundle}" 2>/dev/null | grep -q "[a-zA-Z0-9]"; do sleep 2; done' || (echo "Timeout waiting for validating webhook CA bundle injection" && exit 1)
	@echo "Waiting for the webhook service to be reachable..."
	@timeout 120s bash -c 'until out=$$($(KUBECTL) create --dry-run=server -f config/samples/rabbitmq_v1beta1_rabbitmqcluster.yaml 2>&1); do if echo "$$out" | grep -qE "connection refused|x509|certificate"; then sleep 2; else break; fi; done' || (echo "Timeout waiting for webhook service" && exit 1)

cert-manager: cmctl ## Setup cert-manager. Use CERT_MANAGER_VERSION to customise the version e.g. CERT_MANAGER_VERSION="v1.15.1"
	@echo "Installing Cert Manager"
	$(KUBECTL) apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
	"$(CMCTL)" check api --wait=5m --namespace cert-manager

.PHONY: cert-manager-rm
cert-manager-rm: ## Delete Cert Manager deployment
	@echo "Deleting Cert Manager"
	$(KUBECTL) delete -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml --ignore-not-found

system-tests: install-tools ## Run system tests against Kubernetes cluster defined in ~/.kube/config
	NAMESPACE="$(SYSTEM_TEST_NAMESPACE)" K8S_OPERATOR_NAMESPACE="$(K8S_OPERATOR_NAMESPACE)" RABBITMQ_SERVICE_TYPE="$(RABBITMQ_SERVICE_TYPE)" $(GINKGO_CLI) -nodes=3 --randomize-all -r $(GINKGO_EXTRA) test/system/

##@ E2E Testing

# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# CertManager is installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
KIND_CLUSTER ?= cluster-operator-e2e

.PHONY: setup-test-e2e
setup-test-e2e: kind ## Set up a Kind cluster for e2e tests if it does not exist
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

kubectl-plugin-tests: ## Run kubectl-rabbitmq tests
	@echo "running kubectl plugin tests"
	PATH=$(PWD)/bin:$$PATH ./bin/kubectl-rabbitmq.bats

.PHONY: tests
tests::unit-tests ## Runs all test suites: unit, integration, system and kubectl-plugin
tests::integration-tests
tests::system-tests
tests::kubectl-plugin-tests

docker-registry-secret: ## Create docker registry secret in K8s cluster
	@$(call check_defined, DOCKER_REGISTRY_SERVER, URL of docker registry containing the Operator image e.g. registry.my-company.com)
	@$(call check_defined, DOCKER_REGISTRY_USERNAME, Username for accessing the docker registry e.g. robot-123)
	@$(call check_defined, DOCKER_REGISTRY_PASSWORD, Password for accessing the docker registry e.g. password)
	@$(call check_defined, DOCKER_REGISTRY_SECRET, Name of Kubernetes secret in which to store the Docker registry username and password)
	@printf "creating registry secret and patching default service account"
	@$(KUBECTL) -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) --docker-server='$(DOCKER_REGISTRY_SERVER)' --docker-username="$$DOCKER_REGISTRY_USERNAME" --docker-password="$$DOCKER_REGISTRY_PASSWORD" || true
	@$(KUBECTL) -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount rabbitmq-cluster-operator -p '{"imagePullSecrets": [{"name": "$(DOCKER_REGISTRY_SECRET)"}]}'
