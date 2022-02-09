#!/bin/bash

set -euo pipefail

RABBITMQ_NAMESPACE=${RABBITMQ_NAMESPACE:-'examples'}

vault_exec () {
    kubectl exec vault-0 -c vault -- /bin/sh -c "$*"
}

echo "Installing Vault server and Vault agent injector..."
helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update
# For OpenShift deployments, also set the following:
# --set "global.openshift=true"
helm install vault hashicorp/vault \
    --version 0.19.0 \
    --set='server.dev.enabled=true' \
    --set='server.logLevel=debug' \
    --set='injector.logLevel=debug' \
    --wait
sleep 5
kubectl wait --for=condition=Ready pod/vault-0

echo "Configuring K8s authentication..."
# Required so that Vault init container and sidecar of RabbitmqCluster can authenticate with Vault.
vault_exec "vault auth enable kubernetes"

# In Kubernetes 1.21+ clusters, issuer may need to be configured as described in https://www.vaultproject.io/docs/auth/kubernetes#discovering-the-service-account-issuer
# Otherwise, vault-agent-init container will output "error authenticating".
# issuer=$(kubectl get --raw=http://127.0.0.1:8001/.well-known/openid-configuration | jq -r .issuer)
# vault_exec "vault write auth/kubernetes/config issuer=\"$issuer\" token_reviewer_jwt=\"\$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)\" kubernetes_host=https://\${KUBERNETES_PORT_443_TCP_ADDR}:443 kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
vault_exec "vault write auth/kubernetes/config token_reviewer_jwt=\"\$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)\" kubernetes_host=https://\${KUBERNETES_PORT_443_TCP_ADDR}:443 kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

echo "Creating credentials for rabbitmq default user..."
# Each RabbitMQ cluster may have its own secret path. Here we have a generic path secret/rabbitmq/config
vault_exec "vault kv put secret/rabbitmq/config username='rabbitmq' password='pwd1'"

# Create a policy that allows to read the default user credentials.
# The path need to be referenced from the RabbitmqCluster CRD spec.secretBackend.vault.defaultUserPath
vault_exec "vault policy write rabbitmq-policy - <<EOF
path \"secret/data/rabbitmq/config\" {
    capabilities = [\"read\"]
}
EOF
"

# Define a Vault role that need to be referenced from the RabbitmqCluster CRD spec.secretBackend.vault.role
# Assign the previously created policy.
# bound_service_account_names must be RabbitmqCluster's service account name.
# Service account name follows the pattern "<RabbitmqCluster name>-server”.
# bound_service_account_namespaces must be the namespace where the RabbitmqCluster will be deployed.
vault_exec "vault write auth/kubernetes/role/rabbitmq bound_service_account_names=vault-default-user-server,vault-tls-server bound_service_account_namespaces=$RABBITMQ_NAMESPACE policies=rabbitmq-policy ttl=24h"
