# runs the target list by default
.DEFAULT_GOAL = list

.PHONY: list

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
	$(CONTROLLER_GEN) object:headerFile=./hack/NOTICE.go.txt paths=./api/...
	$(CONTROLLER_GEN) object:headerFile=./hack/NOTICE.go.txt paths=./internal/status/...

# Build manager binary
manager: generate fmt vet
	go mod tidy
	go build -o bin/manager main.go

deploy-manager:  ## Deploy manager
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/base | kubectl apply -f -

deploy-manager-dev:
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/overlays/dev | kubectl apply -f -

deploy-sample: ## Deploy RabbitmqCluster defined in config/sample/base
	kustomize build config/samples/base | kubectl apply -f -

destroy: ## Cleanup all controller artefacts
	kustomize build config/crd/ | kubectl delete --ignore-not-found=true -f -
	kustomize build config/default/base/ | kubectl delete --ignore-not-found=true -f -
	kustomize build config/rbac/ | kubectl delete --ignore-not-found=true -f -
	kustomize build config/namespace/base/ | kubectl delete --ignore-not-found=true -f -

run: generate manifests fmt vet install deploy-namespace-rbac just-run ## Run operator binary locally against the configured Kubernetes cluster in ~/.kube/config

just-run: ## Just runs 'go run main.go' without regenerating any manifests or deploying RBACs
	KUBE_CONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=rabbitmq-system go run ./main.go

delve: generate install deploy-namespace-rbac just-delve ## Deploys CRD, Namespace, RBACs and starts Delve debugger

just-delve: ## Just starts Delve debugger
	KUBE_CONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=rabbitmq-system dlv debug

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd/bases

deploy-namespace-rbac:
	kustomize build config/namespace/base | kubectl apply -f -
	kustomize build config/rbac | kubectl apply -f -

deploy: manifests deploy-namespace-rbac deploy-manager ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config

deploy-dev: docker-build-dev patch-dev manifests deploy-namespace-rbac docker-registry-secret deploy-manager-dev ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config, with local changes

deploy-kind: git-commit-sha patch-kind manifests deploy-namespace-rbac ## Load operator image and deploy operator into current KinD cluster
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	kind load docker-image $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/overlays/kind | kubectl apply -f -

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
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):latest .

# Push the docker image
docker-push:
	docker push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):latest

git-commit-sha:
ifeq ("", git diff --stat)
GIT_COMMIT="$(shell git rev-parse --short HEAD)"
else
GIT_COMMIT="$(shell git rev-parse --short HEAD)-"
endif

docker-build-dev: git-commit-sha
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	docker push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)

patch-dev: git-commit-sha
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"'@' ./config/default/overlays/dev/manager_image_patch.yaml

patch-kind: git-commit-sha
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"'@' ./config/default/overlays/kind/manager_image_patch.yaml

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
	NAMESPACE="rabbitmq-system" ginkgo -nodes=3 -randomizeAllSpecs -r system_tests/


docker-registry-secret: operator-namespace
	echo "creating registry secret and patching default service account"
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) --docker-server='$(DOCKER_REGISTRY_SERVER)' --docker-username="$$DOCKER_REGISTRY_USERNAME" --docker-password="$$DOCKER_REGISTRY_PASSWORD" || true
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount rabbitmq-cluster-operator -p '{"imagePullSecrets": [{"name": "$(DOCKER_REGISTRY_SECRET)"}]}'

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
ifeq (, $(GOBIN))
GOBIN=$(GOPATH)/bin
endif
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

operator-namespace:
ifeq (, $(K8S_OPERATOR_NAMESPACE))
K8S_OPERATOR_NAMESPACE=rabbitmq-system
endif
