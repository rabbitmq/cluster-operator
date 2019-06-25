
# Image URL to use all building/pushing image targets
CONTROLLER_IMAGE=eu.gcr.io/cf-rabbitmq-for-k8s-bunny/rabbitmq-for-kubernetes-controller
CONTROLLER_IMAGE_LOCAL=cf-rabbitmq-for-k8s-bunny/rabbitmq-for-kubernetes-controller
CI_IMAGE=eu.gcr.io/cf-rabbitmq-for-k8s-bunny/rabbitmq-for-kubernetes-ci
CI_CLUSTER=dev-bunny
GCP_PROJECT=cf-rabbitmq-for-k8s-bunny
RABBITMQ_USERNAME=guest
RABBITMQ_PASSWORD=guest

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Run unit tests
unit-tests: generate fmt vet manifests
	ginkgo -r api/ internal/

# Run integration tests
integration-tests: generate fmt vet manifests
	ginkgo -r controllers/

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Deploy manager
deploy-manager:
	kubectl apply -k config/default/base

# Deploy manager in CI
deploy-manager-ci:
	kubectl apply -k config/default/overlays/ci

# Deploy local rabbitmqcluster
deploy-sample:
	kubectl apply -k config/samples/base

# Deploy CI rabbitmqcluster
deploy-sample-ci:
	kustomize build config/samples/overlays/ci | kapp deploy -y -a rabbitmqcluster -f -

configure-kubectl-ci:
	gcloud auth activate-service-account --key-file=$(KUBECTL_SECRET_TOKEN_PATH)
	gcloud container clusters get-credentials $(CI_CLUSTER) --region europe-west1-b --project $(GCP_PROJECT)

# Cleanup all controller artefacts
destroy:
	kubectl delete -k config/default/base
	kubectl delete -k config/namespace/base

destroy-ci: configure-kubectl-ci
	kubectl delete -k config/default/overlays/ci --ignore-not-found=true
	kubectl delete -k config/namespace/overlays/ci

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd/bases

deploy-namespace:
	kubectl apply -k config/namespace/base

deploy-namespace-ci:
	kubectl apply -k config/namespace/overlays/ci

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests deploy-namespace gcr-viewer deploy-manager deploy-sample

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy-ci: configure-kubectl-ci manifests deploy-namespace-ci deploy-manager-ci deploy-sample-ci

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
docker-build:
	docker build . -t ${CONTROLLER_IMAGE}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${CONTROLLER_IMAGE}"'@' ./config/default/base/manager_image_patch.yaml

docker-build-local:
	docker build . -t ${CONTROLLER_IMAGE_LOCAL}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${CONTROLLER_IMAGE_LOCAL}"'@' ./config/default/base/manager_image_patch.yaml
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: IfNotPresent@' ./config/manager/manager.yaml

docker-build-ci-image:
	docker build ci/ -t ${CI_IMAGE}
	docker push ${CI_IMAGE}

# Push the docker image
docker-push:
	docker push ${CONTROLLER_IMAGE}

system-tests: fetch-service-ip
	SERVICE_HOST=$(SERVICE_HOST) NAMESPACE="pivotal-rabbitmq-system" ginkgo -r system_tests/

system-tests-ci: fetch-service-ip-ci
	SERVICE_HOST=$(SERVICE_HOST_CI) NAMESPACE="pivotal-rabbitmq-system-ci" ginkgo -r system_tests/

GCR_VIEWER_KEY=$(shell lpass show "Shared-RabbitMQ for Kubernetes/ci-gcr-pull" --notes | jq -c)
GCR_VIEWER_ACCOUNT_EMAIL='gcr-viewer@cf-rabbitmq-for-k8s-bunny.iam.gserviceaccount.com'
GCR_VIEWER_ACCOUNT_NAME='gcr-viewer'
K8S_OPERATOR_NAMESPACE='pivotal-rabbitmq-system'
gcr-viewer:
	@kubectl -n $(K8S_OPERATOR_NAMESPACE) create secret docker-registry $(GCR_VIEWER_ACCOUNT_NAME) --docker-server=https://eu.gcr.io --docker-username=_json_key --docker-email=$(GCR_VIEWER_ACCOUNT_EMAIL) --docker-password='$(GCR_VIEWER_KEY)' || true
	kubectl -n $(K8S_OPERATOR_NAMESPACE) patch serviceaccount default -p '{"imagePullSecrets": [{"name": "$(GCR_VIEWER_ACCOUNT_NAME)"}]}'


# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0-beta.1
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# TODO - We have temporarily hard coded the ci suffix until we modularize our labels [https://www.pivotaltracker.com/story/show/166494390]
fetch-service-ip:
ifeq ($(SERVICE_HOST),)
SERVICE_HOST=$(shell kubectl -n pivotal-rabbitmq-system get svc -l app=rabbitmqcluster-sample -o jsonpath='{.items[0].status.loadBalancer.ingress[0].ip}')
endif

fetch-service-ip-ci:
ifeq ($(SERVICE_HOST_CI),)
SERVICE_HOST_CI=$(shell kubectl -n pivotal-rabbitmq-system-ci get svc -l app=rabbitmqcluster-sample-ci -o jsonpath='{.items[0].status.loadBalancer.ingress[0].ip}')
endif
