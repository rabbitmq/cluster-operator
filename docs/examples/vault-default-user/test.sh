#!/bin/bash
set -eo pipefail

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

echo "Checking rabbitmqadmin CLI can authenticate..."
kubectl exec vault-default-user-server-0 -c rabbitmq -- \
    rabbitmqadmin show overview

echo "Rotating password in Vault..."
kubectl exec vault-0 -c vault -- vault kv put secret/rabbitmq/config username='rabbitmq' password='pwd2'

# It takes 15 seconds until Vault sidecar detects the password change.
# Can be reduced using Vault Agent annotation "vault.hashicorp.com/template-static-secret-render-interval".
with_retry() {
    retries=3
    while ! eval "$1"
    do
        ((retries=retries-1))
        if [[ "$retries" -eq 0 ]]
        then
            echo "Timed out."
            exit 1
        fi
        echo "Retrying in 10 seconds for $retries more time(s)..."
        sleep 10
    done
}

echo "Checking authentication with new password..."
with_retry "kubectl exec vault-default-user-server-0 -c rabbitmq -- rabbitmqctl authenticate_user rabbitmq pwd2"

echo "Checking rabbitmqadmin CLI can authenticate with new password on server-0..."
with_retry "kubectl exec vault-default-user-server-0 -c rabbitmq -- rabbitmqadmin show overview"
echo "Checking rabbitmqadmin CLI can authenticate with new password on server-1..."
with_retry "kubectl exec vault-default-user-server-1 -c rabbitmq -- rabbitmqadmin show overview"
echo "Checking rabbitmqadmin CLI can authenticate with new password on server-2..."
with_retry "kubectl exec vault-default-user-server-2 -c rabbitmq -- rabbitmqadmin show overview"

helm uninstall vault
