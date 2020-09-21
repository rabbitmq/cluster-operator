# runs the target list by default
.DEFAULT_GOAL = list

.PHONY: list

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true, preserveUnknownFields=false, crdVersions=v1"

# Insert a comment starting with '##' after a target, and it will be printed by 'make' and 'make list'
list:    ## list Makefile targets
	@echo "The most used targets: \n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

unit-tests: install-tools generate fmt vet manifests ## Run unit tests
	ginkgo -r api/ internal/

integration-tests: install-tools generate fmt vet manifests ## Run integration tests
	ginkgo -r controllers/

manifests: install-tools ## Generate manifests e.g. CRD, RBAC etc.
	controller-gen $(CRD_OPTIONS) rbac:roleName=operator-role paths="./api/...;./controllers/..." output:crd:artifacts:config=config/crd/bases
	./hack/add-notice-to-yaml.sh config/rbac/role.yaml
# this is temporary workaround due to issue https://github.com/kubernetes/kubernetes/issues/91395
# the hack ensures that "protocal" is a required value where this field is listed as x-kubernetes-list-map-keys
# without the hack, our crd doesn't install on k8s 1.18 because of the issue above
	./hack/patch-crd.sh
	./hack/add-notice-to-yaml.sh config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: install-tools
	controller-gen object:headerFile=./hack/NOTICE.go.txt paths=./api/...
	controller-gen object:headerFile=./hack/NOTICE.go.txt paths=./internal/status/...

# Build manager binary
manager: generate fmt vet
	go mod tidy
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

run: generate manifests fmt vet install deploy-namespace-rbac just-run ## Run operator binary locally against the configured Kubernetes cluster in ~/.kube/config

just-run: ## Just runs 'go run main.go' without regenerating any manifests or deploying RBACs
	KUBE_CONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=rabbitmq-system go run ./main.go

delve: generate install deploy-namespace-rbac just-delve ## Deploys CRD, Namespace, RBACs and starts Delve debugger

just-delve: install-tools ## Just starts Delve debugger
	KUBE_CONFIG=${HOME}/.kube/config OPERATOR_NAMESPACE=rabbitmq-system dlv debug

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd/bases

deploy-namespace-rbac:
	kustomize build config/namespace/base | kubectl apply -f -
	kustomize build config/rbac | kubectl apply -f -

deploy: manifests deploy-namespace-rbac deploy-manager ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config

deploy-dev: check-env-docker-credentials docker-build-dev manifests deploy-namespace-rbac docker-registry-secret deploy-manager-dev ## Deploy operator in the configured Kubernetes cluster in ~/.kube/config, with local changes

deploy-kind: check-env-docker-repo git-commit-sha manifests deploy-namespace-rbac ## Load operator image and deploy operator into current KinD cluster
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	kind load docker-image $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)
	kustomize build config/crd | kubectl apply -f -
	kustomize build config/default/overlays/kind | sed 's@((operator_docker_image))@"$(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)"@' | kubectl apply -f -

# Builds a single-file installation manifest to deploy the Operator
generate-installation-manifest:
	mkdir -p releases
	kustomize build config/installation/ > releases/rabbitmq-cluster-operator.yaml

# Build the docker image
docker-build: check-env-docker-repo git-commit-sha
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):latest .

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
	docker build --build-arg=GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT) .
	docker push $(DOCKER_REGISTRY_SERVER)/$(OPERATOR_IMAGE):$(GIT_COMMIT)

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

system-tests: install-tools ## run end-to-end tests against Kubernetes cluster defined in ~/.kube/config
	NAMESPACE="rabbitmq-system" ginkgo -nodes=3 -randomizeAllSpecs -r system_tests/

docker-registry-secret: check-env-docker-credentials operator-namespace
	echo "creating registry secret and patching default service account"
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(DOCKER_REGISTRY_SECRET) --docker-server='$(DOCKER_REGISTRY_SERVER)' --docker-username="$$DOCKER_REGISTRY_USERNAME" --docker-password="$$DOCKER_REGISTRY_PASSWORD" || true
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount rabbitmq-cluster-operator -p '{"imagePullSecrets": [{"name": "$(DOCKER_REGISTRY_SECRET)"}]}'

install-tools:
	go mod download
	grep _ tools/tools.go | awk -F '"' '{print $$2}' | xargs -t go install

operator-namespace:
ifeq (, $(K8S_OPERATOR_NAMESPACE))
K8S_OPERATOR_NAMESPACE=rabbitmq-system
endif

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
