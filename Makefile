
# Image URL to use all building/pushing image targets
CONTROLLER_IMAGER=eu.gcr.io/cf-rabbitmq-for-k8s-bunny/rabbitmq-for-kubernetes-controller
all: test manager

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out
	ginkgo -r pkg/controller/rabbitmqcluster/

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
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Cleanup all controller artefacts
# destroy: manifests
#    kubectl delete -f config/crds
#    kustomize build config/default | kubectl delete -f -

# Generate code
generate:
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build: test
	docker build . -t ${CONTROLLER_IMAGER}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${CONTROLLER_IMAGER}

GCR_VIEWER_KEY_CONTENT = `cat ~/Desktop/cf-rabbitmq-for-k8s-bunny-875a177ce777.json`
GCR_VIEWER_ACCOUNT_EMAIL='gcr-viewer@cf-rabbitmq-for-k8s-bunny.iam.gserviceaccount.com'
GCR_VIEWER_ACCOUNT='gcr-viewer'
K8S_OPERATOR_NAMESPACE='pivotal-rabbitmq-system'
gcr_viewer:
	kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(GCR_VIEWER_ACCOUNT) --docker-server=https://eu.gcr.io --docker-username=_json_key --docker-email=$(GCR_VIEWER_ACCOUNT_EMAIL) --docker-password="$(GCR_VIEWER_KEY_CONTENT)" || true
	kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount default -p '{"imagePullSecrets": [{"name": "$(GCR_VIEWER_ACCOUNT)"}]}'
