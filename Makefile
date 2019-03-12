
# Image URL to use all building/pushing image targets
IMG ?= controller:latest

ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif

DEP := $(GOPATH)/bin/dep
$(DEP):
	go get -u github.com/golang/dep/cmd/dep

COUNTERFEITER := $(GOPATH)/bin/counterfeiter
$(COUNTERFEITER):
	go get -u "github.com/maxbrunsfeld/counterfeiter"

KUBEBUILDER_VERSION := 1.0.8
PLATFORM := $(shell uname -s | tr '[:upper:]' '[:lower:]')
KUBEBUILDER := $(CURDIR)/bin/kubebuilder_$(KUBEBUILDER_VERSION)
PATH := $(KUBEBUILDER)/bin:$(PATH)
export PATH

TEST_ASSET_KUBECTL := $(KUBEBUILDER)/bin/kubectl
export TEST_ASSET_KUBECTL

TEST_ASSET_KUBE_APISERVER := $(KUBEBUILDER)/bin/kube-apiserver
export TEST_ASSET_KUBE_APISERVER

TEST_ASSET_ETCD := $(KUBEBUILDER)/bin/etcd
export TEST_ASSET_ETCD

$(KUBEBUILDER):
	mkdir -p $(KUBEBUILDER) && \
	curl --silent --fail --location "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/kubebuilder_$(KUBEBUILDER_VERSION)_$(PLATFORM)_amd64.tar.gz" | \
		tar -zxv --directory=$(KUBEBUILDER) --strip-components=1

all: fmt vet test manifests manager

# Run tests
test: generate
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/pivotal/rabbitmq-for-kubernetes/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: deps
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	mv -f config/rbac/* config/default/rbac/
	rm -rf config/rbac

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet: deps
	go vet ./pkg/... ./cmd/...

deps: $(DEP) $(COUNTERFEITER) $(KUBEBUILDER)
	dep ensure -v

# Generate code
generate: deps
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build: fmt vet manifests test
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}
