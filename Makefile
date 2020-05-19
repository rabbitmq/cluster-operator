# runs the target list by default
.DEFAULT_GOAL = list

.PHONY: list

# Image URL to use all building/pushing image targets
CONTROLLER_IMAGE=dev.registry.pivotal.io/p-rabbitmq-for-kubernetes/rabbitmq-for-kubernetes-operator
CI_IMAGE=us.gcr.io/cf-rabbitmq-for-k8s-bunny/rabbitmq-for-kubernetes-ci
GCP_PROJECT=cf-rabbitmq-for-k8s-bunny
RABBITMQ_USERNAME=guest
RABBITMQ_PASSWORD=guest

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true, preserveUnknownFields=false"

# Insert a comment starting with '##' after a target, and it will be printed by 'make' and 'make list'
list:    ## list Makefile targets
	@echo "The most used targets: \n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

unit-tests: generate fmt vet manifests ## Run unit tests
	ginkgo -r api/ internal/


integration-tests: generate fmt vet manifests ## Run integration tests
	ginkgo -r controllers/

manifests: controller-gen ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=operator-role paths="./api/...;./controllers/..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./api/...
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./internal/status/...

# Build manager binary
manager: generate fmt vet
	go mod tidy
	go build -o bin/manager main.go

deploy-manager:  ## Deploy manager
	kubectl apply -k config/crd
	kubectl apply -k config/default/base

# Deploy manager in CI
deploy-manager-ci:
	kubectl apply -k config/crd
	kubectl apply -k config/default/overlays/ci

deploy-manager-dev:
	kubectl apply -k config/crd
	kubectl apply -k config/default/overlays/dev

deploy-sample: ## Deploy RabbitmqCluster defined in config/sample/base
	kubectl apply -k config/samples/base

configure-kubectl-ci: ci-cluster
	gcloud auth activate-service-account --key-file=$(KUBECTL_SECRET_TOKEN_PATH)
	gcloud container clusters get-credentials $(CI_CLUSTER) --region europe-west1 --project $(GCP_PROJECT)

destroy: ## Cleanup all controller artefacts
	kubectl delete -k config/crd/ --ignore-not-found=true
	kubectl delete -k config/default/base/ --ignore-not-found=true
	kubectl delete -k config/rbac/ --ignore-not-found=true
	kubectl delete -k config/namespace/base/ --ignore-not-found=true

destroy-ci: configure-kubectl-ci
	kubectl delete -k config/crd --ignore-not-found=true
	kubectl delete -k config/default/overlays/ci --ignore-not-found=true
	kubectl delete -k config/rbac/ --ignore-not-found=true
	kubectl delete -k config/namespace/base --ignore-not-found=true

run: generate manifests fmt vet install deploy-namespace-rbac just-run ## Run operator binary locally against the configured Kubernetes cluster in ~/.kube/config

just-run: ## Just runs 'go run main.go' without regenerating any manifests or deploying RBACs
	KUBE_CONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=pivotal-rabbitmq-system go run ./main.go

delve: generate install deploy-namespace-rbac just-delve ## Deploys CRD, Namespace, RBACs and starts Delve debugger

just-delve: ## Just starts Delve debugger
	KUBE_CONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=pivotal-rabbitmq-system dlv debug

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd/bases

deploy-namespace-rbac:
	kubectl apply -k config/namespace/base
	kubectl apply -k config/rbac

deploy-master: install deploy-namespace-rbac docker-registry-secret
	kubectl apply -k config/default/base

deploy: manifests deploy-namespace-rbac docker-registry-secret deploy-manager ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config

deploy-dev: docker-build-dev patch-dev manifests deploy-namespace-rbac docker-registry-secret deploy-manager-dev ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config, with local changes

deploy-kind: git-commit-sha patch-kind manifests deploy-namespace-rbac ## Load operator image and deploy operator into current KinD cluster
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(CONTROLLER_IMAGE):$(GIT_COMMIT) .
	kind load docker-image $(CONTROLLER_IMAGE):$(GIT_COMMIT)
	kubectl apply -k config/crd
	kubectl apply -k config/default/overlays/kind

# Deploy operator in the configured Kubernetes cluster in ~/.kube/config
deploy-ci: configure-kubectl-ci patch-controller-image manifests deploy-namespace-rbac docker-registry-secret-ci deploy-manager-ci

generate-installation-manifests:
	mkdir -p installation
	kustomize build config/namespace/base/ > installation/namespace.yaml
	kustomize build config/crd/ > installation/crd.yaml
	kustomize build config/rbac/ > installation/rbac.yaml
	kustomize build config/installation > installation/operator.yaml

generate-helm-manifests:
	kustomize build config/namespace/base/ > charts/operator/templates/namespace.yaml
	kustomize build config/crd/ > charts/operator/templates/crd.yaml
	kustomize build config/rbac/ > charts/operator/templates/rbac.yaml
	kustomize build config/default/overlays/helm/ > charts/operator/templates/deployment.yaml

# Build the docker image
docker-build: git-commit-sha
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(CONTROLLER_IMAGE):latest .

# Push the docker image
docker-push:
	docker push $(CONTROLLER_IMAGE):latest

git-commit-sha:
ifeq ("", git diff --stat)
GIT_COMMIT="$(shell git rev-parse --short HEAD)"
else
GIT_COMMIT="$(shell git rev-parse --short HEAD)-"
endif

docker-build-dev: git-commit-sha
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(CONTROLLER_IMAGE):$(GIT_COMMIT) .
	docker push $(CONTROLLER_IMAGE):$(GIT_COMMIT)

patch-dev: git-commit-sha
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"$(CONTROLLER_IMAGE):$(GIT_COMMIT)"'@' ./config/default/overlays/dev/manager_image_patch.yaml

patch-kind: git-commit-sha
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"$(CONTROLLER_IMAGE):$(GIT_COMMIT)"'@' ./config/default/overlays/kind/manager_image_patch.yaml

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

system-tests: ## run end-to-end tests against Kubernetes cluster defined in ~/.kube/config
	NAMESPACE="pivotal-rabbitmq-system" ginkgo -nodes=3 --randomizeAllSpecs -r system_tests/

DOCKER_REGISTRY_SECRET=p-rmq-registry-access
DOCKER_REGISTRY_SERVER=dev.registry.pivotal.io
DOCKER_REGISTRY_USERNAME_LOCAL=$(shell lpassd show "Shared-RabbitMQ for Kubernetes/pivnet-dev-registry-ci" --notes | jq -r .name)
DOCKER_REGISTRY_PASSWORD_LOCAL=$(shell lpassd show "Shared-RabbitMQ for Kubernetes/pivnet-dev-registry-ci" --notes | jq -r .token)
docker-registry-secret: operator-namespace
	echo "creating registry secret and patching default service account"
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) --docker-server='$(DOCKER_REGISTRY_SERVER)' --docker-username='$(DOCKER_REGISTRY_USERNAME_LOCAL)' --docker-password='$(DOCKER_REGISTRY_PASSWORD_LOCAL)' || true
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount p-rmq-operator -p '{"imagePullSecrets": [{"name": "$(DOCKER_REGISTRY_SECRET)"}]}'

docker-registry-secret-ci: operator-namespace
	echo "creating registry secret and patching default service account"
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) --docker-server='$(DOCKER_REGISTRY_SERVER)' --docker-username="$$DOCKER_REGISTRY_USERNAME" --docker-password="$$DOCKER_REGISTRY_PASSWORD" || true
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount p-rmq-operator -p '{"imagePullSecrets": [{"name": "$(DOCKER_REGISTRY_SECRET)"}]}'

controller-gen:  ## download controller-gen if not in $PATH
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

patch-controller-image:
	$(eval CONTROLER_IMAGE_NAME:=$(CONTROLLER_IMAGE):latest)
ifneq (, $(CONTROLLER_IMAGE_DIGEST))
	$(eval CONTROLER_IMAGE_NAME:=$(CONTROLLER_IMAGE):latest\@$(CONTROLLER_IMAGE_DIGEST))
endif
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${CONTROLER_IMAGE_NAME}"'@' ./config/default/base/manager_image_patch.yaml


operator-namespace:
ifeq (, $(K8S_OPERATOR_NAMESPACE))
K8S_OPERATOR_NAMESPACE=pivotal-rabbitmq-system
endif

ci-cluster:
ifeq (, $(CI_CLUSTER))
CI_CLUSTER=ci-bunny
endif

