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
kubectl -n default exec vault-0 -c vault -- vault kv put secret/rabbitmq/config username='rabbitmq' password='pwd2'

echo "Checking authentication with new password..."
retries=15
while ! kubectl exec vault-default-user-server-0 -c rabbitmq -- rabbitmqctl authenticate_user rabbitmq pwd2
do
    ((retries=retries-1))
    if [[ "$retries" -eq 0 ]]
    then
        echo "Timed out. Password did not update."
        exit 1
    fi
    echo "Password not yet updated"
    sleep 20
done

echo "Checking rabbitmqadmin CLI can authenticate with new password..."
kubectl exec vault-default-user-server-0 -c rabbitmq -- \
    rabbitmqadmin show overview
