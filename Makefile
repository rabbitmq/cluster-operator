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

# We want to tag the image with the commit sha & dirty if there are uncommitted changes
GIT_REF = $$(git rev-parse --short HEAD)
GIT_DIRTY = $$(git diff --quiet || echo "-dirty")
DOCKER_IMAGE_VERSION = $(GIT_REF)$(GIT_DIRTY)

KUBECTL_PATH=$$(which kubectl)
KUBECTL_MINOR=$$(kubectl version --client=true -o json | jq -r .clientVersion.minor)

# This can be either cf-rabbitmq (default) or cf-rabbitmq-core
GCP_PROJECT ?= cf-rabbitmq

GCP_SERVICE_ACCOUNT = rabbitmq-for-kubernetes
GCP_SERVICE_ACCOUNT_DESCRIPTION = k8s operator images (https://github.com/pivotal/rabbitmq-for-kubernetes)
GCP_SERVICE_ACCOUNT_EMAIL = $(GCP_SERVICE_ACCOUNT)@cf-rabbitmq.iam.gserviceaccount.com
GCP_SERVICE_ACCOUNT_KEY_FILE = $(GCP_SERVICE_ACCOUNT).key.json
GCP_SERVICE_ACCOUNT_KEY = $$($(LPASS) show "Shared-PCF RabbitMQ/$(GCP_SERVICE_ACCOUNT_EMAIL)" --notes)
GCP_BUCKET_NAME = eu.artifacts.$(GCP_PROJECT).appspot.com

GCR_VIEWER_ACCOUNT = rmq4k8s-gcr-viewer
GCR_VIEWER_ACCOUNT_DESCRIPTION = k8s operator images read-only access
GCR_VIEWER_ACCOUNT_EMAIL = $(GCR_VIEWER_ACCOUNT)@cf-rabbitmq.iam.gserviceaccount.com
GCR_VIEWER_ACCOUNT_KEY_FILE = $(GCR_VIEWER_ACCOUNT).key.json
GCR_VIEWER_ACCOUNT_KEY = $$($(LPASS) show "Shared-PCF RabbitMQ/$(GCR_VIEWER_ACCOUNT_EMAIL)" --notes)

GKE_CLUSTER_CREATOR = rmq4k8s-cluster-creator
GKE_CLUSTER_CREATOR_DESCRIPTION = creates GKE clusters for CI pipelines
GKE_CLUSTER_CREATOR_EMAIL = $(GKE_CLUSTER_CREATOR)@cf-rabbitmq.iam.gserviceaccount.com
GKE_CLUSTER_CREATOR_KEY_FILE = $(GKE_CLUSTER_CREATOR).key.json
GKE_CLUSTER_CREATOR_KEY = $$($(LPASS) show "Shared-PCF RabbitMQ/$(GKE_CLUSTER_CREATOR_EMAIL)" --notes)

GIT_SSH_KEY = $$($(LPASS) show "Shared-PCF RabbitMQ/pcf-rabbitmq+github@pivotal.io" --notes)

# Private Docker image reference for the RabbitMQ for K8S Operator image
DOCKER_IMAGE := eu.gcr.io/$(GCP_PROJECT)/rabbitmq-k8s-operator-dev
DOCKER_IMAGE_CI = eu.gcr.io/$(GCP_PROJECT)/rabbitmq-k8s-operator

K8S_NAMESPACE = rabbitmq-for-kubernetes
K8S_OPERATOR_NAMESPACE = rabbitmq-for-kubernetes-system

OPERATOR_BIN = tmp/operator
SERVICEBROKER_BIN = tmp/servicebroker

SERVICEBROKER_DIR = servicebroker

### DEPS ###
#

GCLOUD := /usr/local/bin/gcloud
GSUTIL := /usr/local/bin/gsutil
$(GCLOUD) $(GSUTIL):
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

kubebuilder:
	mkdir -p $(KUBEBUILDER) && \
	curl --silent --fail --location $(KUBEBUILDER_URL) | \
	  tar -zxv --directory=$(KUBEBUILDER) --strip-components=1 && \
	ln -s $(KUBEBUILDER)/bin/kubebuilder ./bin/kubebuilder

TEST_ASSET_KUBECTL := $(KUBEBUILDER)/bin/kubectl
export TEST_ASSET_KUBECTL

TEST_ASSET_KUBE_APISERVER := $(KUBEBUILDER)/bin/kube-apiserver
export TEST_ASSET_KUBE_APISERVER

TEST_ASSET_ETCD := $(KUBEBUILDER)/bin/etcd
export TEST_ASSET_ETCD

FLY := $(LOCAL_BIN)/fly
FLY_URL := https://pcf-rabbitmq.ci.cf-app.com/api/v1/cli?arch=amd64&platform=$(PLATFORM)
$(FLY):
	curl --silent --fail --location --output $(FLY) "$(FLY_URL)" && \
	touch $(FLY) && \
	chmod +x $(FLY) && \
	$(FLY) --version

LPASS := /usr/local/bin/lpass
$(LPASS):
	brew install lastpass-cli



### TARGETS ###
#

.DEFAULT_GOAL := help
.PHONY: help
help:
	@grep -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN { FS = "[:#]" } ; { printf "\033[36m%-20s\033[0m %s\n", $$1, $$4 }' | sort

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

$(OPERATOR_BIN): generate fmt vet test manifests tmp ## Build operator binary
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o $(OPERATOR_BIN) github.com/pivotal/rabbitmq-for-kubernetes/cmd/operator

operator: $(OPERATOR_BIN)

$(SERVICEBROKER_BIN): fmt vet test tmp mod_service_broker ## Build broker binary
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o $(SERVICEBROKER_BIN) github.com/pivotal/rabbitmq-for-kubernetes/servicebroker

servicebroker: $(SERVICEBROKER_BIN)

kubectl:
	@if [[ "$(KUBECTL_PATH)" == "" || $(KUBECTL_MINOR) -lt 14 ]]; then \
		echo "You need kubectl 1.14+"; \
		exit 1; \
	fi

.PHONY: build
build: $(OPERATOR_BIN) $(SERVICEBROKER_BIN)

.PHONY: run
run: generate fmt vet ## Run against the currently targeted K8S cluster
	go run ./cmd/operator/main.go

.PHONY: deploy_crds
deploy_crds: kubectl
	kubectl apply -f config/crds

.PHONY: deploy_operator
deploy_operator: kubectl
	kubectl apply -k config/default

.PHONY: patch_operator_image
patch_operator_image: kubectl
	kubectl patch statefulset rabbitmq-for-kubernetes-controller-operator \
	  --patch='{"spec": {"template": {"spec": {"containers": [{"image": "$(shell echo $(DOCKER_IMAGE):$(DOCKER_IMAGE_VERSION))", "name": "operator"}]}}}}' \
	  --namespace=$(K8S_OPERATOR_NAMESPACE) && \
	echo "$(BOLD)Force operator pod to re-create using the new image...$(NORMAL)"
	echo "If image pull fails on first deploy, it won't recover."
	kubectl delete pod/rabbitmq-for-kubernetes-controller-operator-0 --namespace=$(K8S_OPERATOR_NAMESPACE)

.PHONY: patch_operator_image_ci
patch_operator_image_ci: kubectl
	kubectl patch statefulset rabbitmq-for-kubernetes-controller-operator \
	  --patch='{"spec": {"template": {"spec": {"containers": [{"image": "$(shell echo $(DOCKER_IMAGE_CI):latest)", "name": "operator"}]}}}}' \
	  --namespace=$(K8S_OPERATOR_NAMESPACE) && \
	echo "$(BOLD)Force operator pod to re-create using the new image...$(NORMAL)"
	echo "If image pull fails on first deploy, it won't recover."
	kubectl delete pod/rabbitmq-for-kubernetes-controller-operator-0 --namespace=$(K8S_OPERATOR_NAMESPACE)

.PHONY: deploy
deploy: manifests deploy_crds deploy_operator patch_operator_image ## Deploy Operator in the currently targeted K8S cluster

.PHONY: deploy_ci
deploy_ci: deploy_crds deploy_operator patch_operator_image_ci

.PHONY: delete
delete: kubectl ## Delete operator & all deployments
	kubectl delete namespaces $(K8S_OPERATOR_NAMESPACE) ; \
	kubectl delete namespaces $(K8S_NAMESPACE) ; \
	true

namespace: kubectl
	kubectl get namespace $(K8S_NAMESPACE) $(SILENT) || \
	kubectl create namespace $(K8S_NAMESPACE)
	kubectl get namespace $(K8S_OPERATOR_NAMESPACE) $(SILENT) || \
	kubectl create namespace $(K8S_OPERATOR_NAMESPACE)

.PHONY: single
single: kubectl namespace ## Ask Operator to provision a single-node RabbitMQ
	kubectl --namespace=$(K8S_OPERATOR_NAMESPACE) apply --filename=config/samples/test-single.yml --namespace=$(K8S_NAMESPACE)

.PHONY: single_smoke_test
single_smoke_test: kubectl single
	./scripts/wait_for_rabbitmq_cluster test-single-rabbitmq $(K8S_NAMESPACE)
	kubectl --namespace=$(K8S_NAMESPACE) exec -it test-single-rabbitmq-0 rabbitmqctl -- add_user test test || true
	kubectl --namespace=$(K8S_NAMESPACE) exec -it test-single-rabbitmq-0 rabbitmqctl -- set_permissions -p "/"  test '.*' '.*' '.*'
	-kubectl --namespace=$(K8S_NAMESPACE) delete jobs.batch single-smoke-test
	kubectl --namespace=$(K8S_NAMESPACE) create job single-smoke-test --image=pivotalrabbitmq/perf-test -- bin/runjava com.rabbitmq.perf.PerfTest --uri "amqp://test:test@test-single-rabbitmq.rabbitmq-for-kubernetes.svc.cluster.local" --pmessage=100 --rate 10
	@echo "Waiting for smoke tests to complete (timeout is 60 seconds)"
	@kubectl --namespace=$(K8S_NAMESPACE) wait --for=condition=complete job/single-smoke-test --timeout=60s || (echo "Smoke tests failed"; exit 1)
	@kubectl --namespace=$(K8S_NAMESPACE) delete jobs.batch single-smoke-test
	@echo "Smoke tests completed successfully"

.PHONY: ha_smoke_test
ha_smoke_test: kubectl ha
	./scripts/wait_for_rabbitmq_cluster test-ha-rabbitmq $(K8S_NAMESPACE)
	kubectl --namespace=$(K8S_NAMESPACE) exec -it test-ha-rabbitmq-0 rabbitmqctl -- add_user test test || true
	kubectl --namespace=$(K8S_NAMESPACE) exec -it test-ha-rabbitmq-0 rabbitmqctl -- set_permissions -p "/"  test '.*' '.*' '.*'
	-kubectl --namespace=$(K8S_NAMESPACE) delete jobs.batch ha-smoke-test
	kubectl --namespace=$(K8S_NAMESPACE) create job ha-smoke-test --image=pivotalrabbitmq/perf-test -- bin/runjava com.rabbitmq.perf.PerfTest --uri "amqp://test:test@test-ha-rabbitmq.rabbitmq-for-kubernetes.svc.cluster.local" --pmessage=100 --rate 10
	@echo "Waiting for smoke tests to complete (timeout is 60 seconds)"
	@kubectl --namespace=$(K8S_NAMESPACE) wait --for=condition=complete job/ha-smoke-test --timeout=60s || (echo "Smoke tests failed"; exit 1)
	@kubectl --namespace=$(K8S_NAMESPACE) delete jobs.batch ha-smoke-test
	@echo "Smoke tests completed successfully"

.PHONY: single_port_forward
single_port_forward: kubectl ## Ask Operator to provision a single-node RabbitMQ
	@echo "$(BOLD)http://127.0.0.1:15672/#/login/guest/guest$(NORMAL)" && \
	kubectl port-forward service/test-single-rabbitmq 15672:15672 --namespace=$(K8S_NAMESPACE)

.PHONY: single_delete
single_delete: kubectl ## Delete single-node RabbitMQ
	kubectl delete --filename=config/samples/test-single.yml --namespace=$(K8S_NAMESPACE)

.PHONY: ha
ha: kubectl namespace ## Ask Operator to provision for an HA RabbitMQ
	kubectl --namespace=$(K8S_OPERATOR_NAMESPACE) apply --filename=config/samples/test-ha.yml --namespace=$(K8S_NAMESPACE)

.PHONY: ha_delete
ha_delete: kubectl ## Delete HA RabbitMQ
	kubectl delete --filename=config/samples/test-ha.yml --namespace=$(K8S_NAMESPACE)

.PHONY: ha_port_forward
ha_port_forward: kubectl ## Ask Operator to provision a single-node RabbitMQ
	@echo "$(BOLD)http://127.0.0.1:15672/#/login/guest/guest$(NORMAL)" && \
	kubectl port-forward service/test-ha-rabbitmq 15672:15672 --namespace=$(K8S_NAMESPACE)

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

.PHONY: mod
mod_service_broker:
	cd $(SERVICEBROKER_DIR) && \
	GO111MODULE=on go mod vendor


.PHONY: resources
resources: kubectl
	@echo "$(BOLD)$(K8S_NAMESPACE)$(NORMAL)" && \
	kubectl get all --namespace=$(K8S_NAMESPACE) && \
	echo -e "\n$(BOLD)$(K8S_OPERATOR_NAMESPACE)$(NORMAL)" && \
	kubectl get all --namespace=$(K8S_OPERATOR_NAMESPACE)

.PHONY: deps
deps: $(DEP) $(COUNTERFEITER) ## Resolve dependencies
	dep ensure -v

.PHONY: generate
generate: deps ## Generate code
	go generate ./pkg/... ./cmd/...

.PHONY: image_build
image_build: $(OPERATOR_BIN)
	docker build . \
	  --tag $(DOCKER_IMAGE):$(DOCKER_IMAGE_VERSION) \
	  --tag $(DOCKER_IMAGE):latest

.PHONY: image_publish
image_publish:
	docker push $(DOCKER_IMAGE):$(DOCKER_IMAGE_VERSION) && \
	docker push $(DOCKER_IMAGE):latest

.PHONY: image
image: image_build image_publish ## Build & publish Docker image

.PHONY: images
images: $(GCLOUD) ## Show all Docker images stored on GCR
	$(GCLOUD) container images list-tags $(DOCKER_IMAGE) && \
	echo && $(GCLOUD) container images describe $(DOCKER_IMAGE):$(DOCKER_IMAGE_VERSION)

.PHONY: ci
ci: $(FLY) $(LPASS) ## Configure CI
	GIT_SSH_KEY=$(GIT_SSH_KEY) && \
	GCP_SERVICE_ACCOUNT_KEY=$(GCP_SERVICE_ACCOUNT_KEY) && \
	GKE_CLUSTER_CREATOR_KEY=$(GKE_CLUSTER_CREATOR_KEY) && \
	( $(FLY) --target pcf-rabbitmq status || \
	  $(FLY) --target pcf-rabbitmq login --concourse-url https://pcf-rabbitmq.ci.cf-app.com/ ) && \
	$(FLY) --target pcf-rabbitmq set-pipeline \
	  --pipeline rmq-k8s \
	  --var git-ssh-key="$$GIT_SSH_KEY" \
	  --var gcp-service-account-key="$$GCP_SERVICE_ACCOUNT_KEY" \
	  --var gke-cluster-creator-key="$$GKE_CLUSTER_CREATOR_KEY" \
	  --config ci/operator.yml && \
	$(FLY) --target pcf-rabbitmq set-pipeline \
	  --pipeline rmq-k8s-image \
	  --var git-ssh-key="$$GIT_SSH_KEY" \
	  --var gcp-service-account-key="$$GCP_SERVICE_ACCOUNT_KEY" \
	  --config ci/docker-image.yml

.PHONY: service_account
service_account: $(GCLOUD) $(GSUTIL) tmp
	$(GCLOUD) iam service-accounts create $(GCP_SERVICE_ACCOUNT) --display-name="$(GCP_SERVICE_ACCOUNT_DESCRIPTION)" && \
	$(GCLOUD) iam service-accounts keys create --iam-account="$(GCP_SERVICE_ACCOUNT_EMAIL)" tmp/$(GCP_SERVICE_ACCOUNT_KEY_FILE) && \
	$(GSUTIL) iam ch serviceAccount:$(GCP_SERVICE_ACCOUNT_EMAIL):admin gs://$(GCP_BUCKET_NAME)
	# TODO: GKE service account used for smoke tests should be separated from the bucket admin
	$(GCLOUD) projects add-iam-policy-binding cf-rabbitmq --role=roles/container.developer --member=serviceAccount:$(GCP_SERVICE_ACCOUNT_EMAIL)


GCR_VIEWER_KEY_CONTENT = `cat tmp/$(GCR_VIEWER_ACCOUNT_KEY_FILE)`
.PHONY: gcr_viewer_service_account
gcr_viewer_service_account: $(GCLOUD) $(GSUTIL) kubectl tmp namespace
	$(GCLOUD) iam service-accounts create $(GCR_VIEWER_ACCOUNT) --display-name="$(GCR_VIEWER_ACCOUNT_DESCRIPTION)" && \
	$(GCLOUD) iam service-accounts keys create --iam-account="$(GCR_VIEWER_ACCOUNT_EMAIL)" tmp/$(GCR_VIEWER_ACCOUNT_KEY_FILE) && \
	$(GSUTIL) iam ch serviceAccount:$(GCR_VIEWER_ACCOUNT_EMAIL):objectViewer gs://$(GCP_BUCKET_NAME)
	kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(GCR_VIEWER_ACCOUNT) --docker-server=https://eu.gcr.io --docker-username=_json_key --docker-email=$(GCR_VIEWER_ACCOUNT_EMAIL) --docker-password="$(GCR_VIEWER_KEY_CONTENT)"
	kubectl -n $(K8S_OPERATOR_NAMESPACE)  patch serviceaccount default -p '{"imagePullSecrets": [{"name": "$(GCR_VIEWER_ACCOUNT)"}]}'

GKE_CLUSTER_CREATOR_KEY_CONTENT = `cat tmp/$(GKE_CLUSTER_CREATOR_KEY_FILE)`
.PHONY: gke_cluster_creator_service_account
gke_cluster_creator_service_account: $(GCLOUD) $(GSUTIL) tmp namespace
	$(GCLOUD) iam service-accounts create $(GKE_CLUSTER_CREATOR) --display-name="$(GKE_CLUSTER_CREATOR_DESCRIPTION)" && \
	$(GCLOUD) iam service-accounts keys create --iam-account="$(GKE_CLUSTER_CREATOR_EMAIL)" tmp/$(GKE_CLUSTER_CREATOR_KEY_FILE)
	$(GCLOUD) projects add-iam-policy-binding cf-rabbitmq --role=roles/container.clusterAdmin --role=roles/iam.serviceAccountUser --member=serviceAccount:$(GKE_CLUSTER_CREATOR_EMAIL)

tmp:
	mkdir -p tmp
