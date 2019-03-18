SHELL := bash# we want bash behaviour in all shell invocations
PLATFORM := $(shell uname -s | tr '[:upper:]' '[:lower:]')

ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif

SILENT := 1>/dev/null 2>&1

BOLD := $(shell tput bold)
NORMAL := $(shell tput sgr0)



### VARS ###
#

# We use the Pivotal Tracker story ID that we are currently working on.
# As we make progress with the story, we push new images which re-use the story ID tag.
# When the story is delivered, the image tagged with this story ID should be used for acceptance.
DOCKER_IMAGE_VERSION = 164498730

# Since we use multiple projects (cf-rabbitmq & cf-rabbitmq-core),
# we resolve the currently targeted GCP project just-in-time, from the local env.
define GCP_PROJECT
$$($(GCLOUD) config get-value project)
endef

# Private Docker image reference for the RabbitMQ for K8S Manager image
define DOCKER_IMAGE
eu.gcr.io/$(GCP_PROJECT)/rabbitmq-k8s-manager
endef

K8S_NAMESPACE ?= rabbitmq-for-kubernetes
K8S_MANAGER_NAMESPACE = rabbitmq-for-kubernetes-system



### DEPS ###
#

GCLOUD := /usr/local/bin/gcloud
$(GCLOUD):
	brew cask install google-cloud-sdk

DEP := $(GOPATH)/bin/dep
$(DEP):
	go get -u github.com/golang/dep/cmd/dep

COUNTERFEITER := $(GOPATH)/bin/counterfeiter
$(COUNTERFEITER):
	go get -u github.com/maxbrunsfeld/counterfeiter

LOCAL_BIN := $(CURDIR)/bin
PATH := $(LOCAL_BIN):$(PATH)
export PATH

KUBEBUILDER_VERSION := 1.0.8
KUBEBUILDER_URL := https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/kubebuilder_$(KUBEBUILDER_VERSION)_$(PLATFORM)_amd64.tar.gz
KUBEBUILDER := $(LOCAL_BIN)/kubebuilder_$(KUBEBUILDER_VERSION)
PATH := $(KUBEBUILDER)/bin:$(PATH)
export PATH

$(KUBEBUILDER):
	mkdir -p $(KUBEBUILDER) && \
	curl --silent --fail --location $(KUBEBUILDER_URL) | \
	  tar -zxv --directory=$(KUBEBUILDER) --strip-components=1

TEST_ASSET_KUBECTL := $(KUBEBUILDER)/bin/kubectl
export TEST_ASSET_KUBECTL

TEST_ASSET_KUBE_APISERVER := $(KUBEBUILDER)/bin/kube-apiserver
export TEST_ASSET_KUBE_APISERVER

TEST_ASSET_ETCD := $(KUBEBUILDER)/bin/etcd
export TEST_ASSET_ETCD

KUSTOMIZE_VERSION := 2.0.3
KUSTOMIZE_URL := https://github.com/kubernetes-sigs/kustomize/releases/download/v$(KUSTOMIZE_VERSION)/kustomize_$(KUSTOMIZE_VERSION)_$(PLATFORM)_amd64
KUSTOMIZE := $(LOCAL_BIN)/kustomize_$(KUSTOMIZE_VERSION)
$(KUSTOMIZE):
	curl --silent --fail --location --output $(KUSTOMIZE) "$(KUSTOMIZE_URL)" && \
	touch $(KUSTOMIZE) && \
	chmod +x $(KUSTOMIZE) && \
	($(KUSTOMIZE) version | grep $(KUSTOMIZE_VERSION)) && \
	ln -sf $(KUSTOMIZE) $(CURDIR)/bin/kustomize



### TARGETS ###
#

.DEFAULT_GOAL := help
.PHONY: help
help:
	@grep -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN { FS = "[:#]" } ; { printf "\033[36m%-12s\033[0m %s\n", $$1, $$4 }' | sort

.PHONY: env
env: ## Set shell environment to run commands - eval "$(make env)"
	export PATH=$(PATH)

.PHONY: test_env
test_env: ## Set shell environment required to run tests - eval "$(make test_env)"
	export TEST_ASSET_KUBECTL=$(TEST_ASSET_KUBECTL)
	export TEST_ASSET_KUBE_APISERVER=$(TEST_ASSET_KUBE_APISERVER)
	export TEST_ASSET_ETCD=$(TEST_ASSET_ETCD)

.PHONY: test
test: generate ## Run tests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: manager
manager: generate fmt vet test manifests ## Build manager binary
	go build -o bin/manager github.com/pivotal/rabbitmq-for-kubernetes/cmd/manager

.PHONY: run
run: generate fmt vet ## Run against the currently targeted K8S cluster
	go run ./cmd/manager/main.go

.PHONY: deploy_crds
deploy_crds: manifests
	kubectl apply -f config/crds

.PHONY: deploy_manager
deploy_manager: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: patch_manager_image
patch_manager_image:
	kubectl patch statefulset rabbitmq-for-kubernetes-controller-manager \
	  --patch='{"spec": {"template": {"spec": {"containers": [{"image": "$(shell echo $(DOCKER_IMAGE):$(DOCKER_IMAGE_VERSION))", "name": "manager"}]}}}}' \
	  --namespace=$(K8S_MANAGER_NAMESPACE)

.PHONY: deploy
deploy: deploy_crds deploy_manager patch_manager_image ## Deploy Manager in the currently targeted K8S cluster

namespace:
	kubectl get namespace $(K8S_NAMESPACE) $(SILENT) || \
	kubectl create namespace $(K8S_NAMESPACE)

.PHONY: single
single: namespace ## Ask Manager to provision a single-node RabbitMQ
	kubectl apply --filename=config/samples/test-single.yml --namespace=$(K8S_NAMESPACE)

.PHONY: ha
ha: namespace ## Ask Manager to provision for an HA RabbitMQ
	kubectl apply --filename=config/samples/test-ha.yml --namespace=$(K8S_NAMESPACE)

.PHONY: delete
delete: ## Delete all Manager resources
	kubectl delete namespaces $(K8S_MANAGER_NAMESPACE)

.PHONY: manifests
manifests: deps ## Generate manifests e.g. CRD, RBAC etc.
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all && \
	mv -f config/rbac/* config/default/rbac/ && \
	rm -rf config/rbac

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet: deps ## Run go vet against code
	go vet ./pkg/... ./cmd/...

.PHONY: resources
resources:
	@echo "$(BOLD)$(K8S_NAMESPACE)$(NORMAL)" && \
	kubectl get all --namespace=$(K8S_NAMESPACE) && \
	echo -e "\n$(BOLD)$(K8S_MANAGER_NAMESPACE)$(NORMAL)" && \
	kubectl get all --namespace=$(K8S_MANAGER_NAMESPACE)

.PHONY: deps
deps: $(DEP) $(COUNTERFEITER) $(KUBEBUILDER) ## Resolve dependencies
	dep ensure -v

.PHONY: generate
generate: deps ## Generate code
	go generate ./pkg/... ./cmd/...

.PHONY: image_build
image_build: fmt vet test manifests
	docker build . \
	  --tag $(DOCKER_IMAGE):$(DOCKER_IMAGE_VERSION)
	  --tag $(DOCKER_IMAGE):latest

.PHONY: image_publish
image_publish:
	docker push $(DOCKER_IMAGE):$(DOCKER_IMAGE_VERSION) && \
	docker push $(DOCKER_IMAGE):latest

.PHONY: image
image: image_build image_publish ## Build & publish Docker image

.PHONY: images
images: ## Show all Docker images stored on GCR
	$(GCLOUD) container images list-tags $(DOCKER_IMAGE) && \
	echo && $(GCLOUD) container images describe $(DOCKER_IMAGE):$(DOCKER_IMAGE_VERSION)
