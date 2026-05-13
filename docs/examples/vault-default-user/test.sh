#!/bin/bash
set -eo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck source=../../../scripts/with_retry.sh
source "$SCRIPT_DIR/../../../scripts/with_retry.sh"

echo "Checking 'rabbitmq' user exists..."
USERNAME=$(kubectl exec vault-default-user-server-0 -c rabbitmq -- \
    rabbitmqctl list_users --formatter=json | jq -r .[0].user)
if [ "$USERNAME" = "rabbitmq" ]; then
    echo "user 'rabbitmq' exists"
else
    echo "user 'rabbitmq' does not exist"
    exit 1
fi

echo "Checking default-user K8s secret is absent..."
secrets="$(kubectl get secrets -l=app.kubernetes.io/name=vault-default-user -o json | jq .items)"
length="$(echo "$secrets" | jq length)"
[ "$length" = 1 ] || (echo "expected 1 secret, but got $length secrets" && exit 1)
[ "$(echo "$secrets" | jq -r '.[0].metadata.name')" = 'vault-default-user-erlang-cookie' ]

echo "Checking authentication works..."
kubectl exec vault-default-user-server-0 -c rabbitmq -- \
    rabbitmqctl authenticate_user rabbitmq pwd1

echo "Rotating password in Vault..."
kubectl exec vault-0 -c vault -- vault kv put secret/rabbitmq/config username='rabbitmq' password='pwd2'

# It takes 15 seconds until Vault sidecar detects the password change.
# Can be reduced using Vault Agent annotation "vault.hashicorp.com/template-static-secret-render-interval".

echo "Checking authentication with new password..."
with_retry "kubectl exec vault-default-user-server-0 -c rabbitmq -- rabbitmqctl authenticate_user rabbitmq pwd2"

helm uninstall vault
