
# Image URL to use all building/pushing image targets
CONTROLLER_IMAGE=eu.gcr.io/cf-rabbitmq-for-k8s-bunny/rabbitmq-for-kubernetes-controller

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./api/... ./controllers/... -coverprofile cover.out
	ginkgo -r pkg/controller/rabbitmqcluster/

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Deploy manager
deploy-manager:
	kustomize build config/default | kubectl apply -f -

# Deploy namespace
deploy-namespace:
	kubectl apply -f config/manager/namespace.yaml

# Cleanup all controller artefacts
destroy:
	kustomize build config/default | kubectl delete -f -
	kubectl delete -f config/crds
	kubectl delete -f config/manager/namespace.yaml

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd/bases

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests install deploy-namespace gcr-viewer deploy-manager

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./api/...;./controllers/..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./api/...

# Build the docker image
docker-build: test
	docker build . -t ${CONTROLLER_IMAGE}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${CONTROLLER_IMAGE}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${CONTROLLER_IMAGE}

GCR_VIEWER_KEY_CONTENT = `cat ~/Desktop/cf-rabbitmq-for-k8s-bunny-875a177ce777.json`
GCR_VIEWER_ACCOUNT_EMAIL='gcr-viewer@cf-rabbitmq-for-k8s-bunny.iam.gserviceaccount.com'
GCR_VIEWER_ACCOUNT='gcr-viewer'
K8S_OPERATOR_NAMESPACE='pivotal-rabbitmq-system'
gcr-viewer:
	kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(GCR_VIEWER_ACCOUNT) --docker-server=https://eu.gcr.io --docker-username=_json_key --docker-email=$(GCR_VIEWER_ACCOUNT_EMAIL) --docker-password="$(GCR_VIEWER_KEY_CONTENT)" || true
	kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount default -p '{"imagePullSecrets": [{"name": "$(GCR_VIEWER_ACCOUNT)"}]}'


# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0-beta.1
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
