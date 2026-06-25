#!/usr/bin/env bash
set -euo pipefail

echo "==> Loading environment variables..."
source .envrc

export REGISTRY="${DOCKER_REGISTRY_SERVER}"
export OPERATOR_IMG="${REGISTRY}/${DOCKER_REGISTRY_USERNAME}/cluster-operator:latest"
export BUNDLE_IMG_NAME="${DOCKER_REGISTRY_USERNAME}/cluster-operator-bundle:latest"
export CATALOG_IMG_NAME="${DOCKER_REGISTRY_USERNAME}/cluster-operator-catalog:latest"

echo "==> Building and pushing Operator image..."
make docker-build docker-push IMG="${OPERATOR_IMG}"

echo "==> Generating OLM manifests..."
make -f olm.mk all QUAY_IO_OPERATOR_IMAGE="${OPERATOR_IMG}" BUNDLE_REPLACES="rabbitmq-cluster-operator.v0.0.0"

echo "==> Building and pushing bundle image..."
make -f olm.mk docker-build docker-push REGISTRY="${REGISTRY}" IMAGE="${BUNDLE_IMG_NAME}"

echo "==> Generating catalog manifests..."
make -f olm.mk catalog-replace-bundle REGISTRY="${REGISTRY}" IMAGE="${BUNDLE_IMG_NAME}"

echo "==> Building and pushing catalog image..."
make -f olm.mk catalog-build catalog-push REGISTRY="${REGISTRY}" CATALOG_IMAGE="${CATALOG_IMG_NAME}"

echo "==> Setting up pull secrets..."
make -f olm.mk catalog-pull-secret

echo "==> Patching service accounts in ns-1 to use the image pull secret..."
# The operator pod will need this secret to pull the operator image
kubectl patch serviceaccount default -n ns-1 -p "{\"imagePullSecrets\": [{\"name\": \"${DOCKER_REGISTRY_SECRET}\"}]}" || true

echo "==> Deploying catalog and subscription..."
make -f olm.mk catalog-deploy REGISTRY="${REGISTRY}" CATALOG_IMAGE="${CATALOG_IMG_NAME}"

echo "==> Waiting for CatalogSource pod to be created..."
retries=30
while [[ $retries -gt 0 ]]; do
    POD_NAME=$(kubectl get pod -l olm.catalogSource=cool-catalog -n ns-1 --field-selector=status.phase!=Terminating -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [[ -n "$POD_NAME" ]]; then
        echo "Found CatalogSource pod: $POD_NAME"
        break
    fi
    echo "Waiting for CatalogSource pod..."
    sleep 5
    retries=$((retries - 1))
done

echo "==> Waiting for CatalogSource pod to be ready..."
kubectl wait --for=condition=ready pod -l olm.catalogSource=cool-catalog -n ns-1 --timeout=120s || {
    echo "CatalogSource pod failed to become ready. Fetching logs..."
    kubectl get pods -n ns-1
    kubectl describe catalogsource cool-catalog -n ns-1
    exit 1
}

echo "==> Waiting for Subscription to generate CSV..."
retries=30
CSV_NAME=""
while [[ $retries -gt 0 ]]; do
    CSV_NAME=$(kubectl get sub rabbitmq-cluster-operator -n ns-1 -o jsonpath='{.status.installedCSV}' 2>/dev/null || echo "")
    if [[ -n "$CSV_NAME" ]]; then
        echo "Found CSV: $CSV_NAME"
        break
    fi
    echo "Waiting for CSV to be created..."
    sleep 5
    retries=$((retries - 1))
done

if [[ -z "$CSV_NAME" ]]; then
    echo "Timed out waiting for Subscription to create CSV."
    kubectl get sub rabbitmq-cluster-operator -n ns-1 -o yaml
    exit 1
fi

echo "==> Patching ServiceAccount created by OLM..."
# OLM overwrites the SA, so we patch it after CSV creation
kubectl patch serviceaccount rabbitmq-cluster-operator -n ns-1 -p "{\"imagePullSecrets\": [{\"name\": \"${DOCKER_REGISTRY_SECRET}\"}]}" || true
# Delete the failing pod so it gets recreated with the new SA credentials
kubectl delete pod -l app.kubernetes.io/name=rabbitmq-cluster-operator -n ns-1 || true

echo "==> Waiting for CSV to succeed..."
kubectl wait --for=condition=Succeeded csv/"$CSV_NAME" -n ns-1 --timeout=120s || {
    echo "CSV failed to succeed."
    kubectl get csv/"$CSV_NAME" -n ns-1 -o yaml
    kubectl get pods -n ns-1
    exit 1
}

echo "==> Verifying Operator Pods..."
kubectl get pods -n ns-1

echo "==> E2E Test Completed Successfully!"
