SHELL = /bin/sh
PLATFORM := $(shell uname | tr A-Z a-z)
ARCHITECTURE := $(shell uname -m)
ifeq ($(ARCHITECTURE),x86_64)
	ARCHITECTURE=amd64
endif

ifeq ($(ARCHITECTURE),aarch64)
	ARCHITECTURE=arm64
endif

.DEFAULT: all

all::crd
all::rbac
all::deployment
all::olm-manifests

.PHONY: all crd rbac deployment olm-manifests clean

crd:
	kustomize build config/crd > olm/manifests/rabbitmq.com_rabbitmqcluster.yaml

rbac:
	mkdir -p tmp/
	yq '{"rules": .rules}' config/rbac/role.yaml > tmp/role-rules.yaml

QUAY_IO_OPERATOR_IMAGE ?= quay.io/rabbitmqoperator/cluster-operator:latest
deployment:
	mkdir -p tmp/
	kustomize build config/installation/ | \
		ytt -f- -f config/ytt/overlay-manager-image.yaml --data-value operator_image=$(QUAY_IO_OPERATOR_IMAGE) \
			-f olm/templates/cluster-operator-namespace-scope-overlay.yml \
		> tmp/cluster-operator.yml
	yq '{"spec": .spec}' tmp/cluster-operator.yml > tmp/spec.yaml

OLM_DIR = olm/manifests
$(OLM_DIR) :
	mkdir -pv $@

OLM_CREATED_AT ?= $(shell date +'%Y-%m-%dT%H:%M:%S')
OLM_VERSION ?= 0.0.0
olm-manifests:
	ytt -f olm/templates/cluster-service-version-generator-openshift.yml \
		--data-values-file tmp/role-rules.yaml \
		--data-values-file tmp/spec.yaml \
		--data-value name="rabbitmq-cluster-operator" \
		--data-value createdAt="$(OLM_CREATED_AT)" \
		--data-value image="$(QUAY_IO_OPERATOR_IMAGE)" \
		--data-value version="$(OLM_VERSION)" \
		> $(OLM_DIR)/rabbitmq-cluster-operator.clusterserviceversion.yaml

clean:
	rm -f -v olm/manifests/*.y*ml

###########
## Build ##
###########

CONTAINER ?= docker
REGISTRY ?= quay.io
IMAGE ?= rabbitmqoperator/rabbitmq-for-kubernetes-olm-cluster-operator:latest

.PHONY: docker-build docker-push
docker-build:
	$(CONTAINER) build -t $(REGISTRY)/$(IMAGE) -f olm/bundle.Dockerfile ./olm

docker-push:
	$(CONTAINER) push $(REGISTRY)/$(IMAGE)

