#!/bin/bash
set -eo pipefail

USERNAME=$(kubectl exec -it vault-default-user-server-0 -c rabbitmq -- \
    rabbitmqctl list_users --formatter=json | jq -r .[0].user)
if [ "$USERNAME" = "rabbitmq" ]; then
    echo "user 'rabbitmq' exists"
else
    echo "user 'rabbitmq' does not exist"
    exit 1
fi

# Check that the default-user K8s Secret is absent.
secrets="$(kubectl get secrets -l=app.kubernetes.io/name=vault-default-user -o json | jq .items)"
length="$(echo "$secrets" | jq length)"
[ "$length" = 1 ] || (echo "expected 1 secret, but got $length secrets" && exit 1)
[ "$(echo "$secrets" | jq -r '.[0].metadata.name')" = 'vault-default-user-erlang-cookie' ]
