#!/bin/bash
set -xeo pipefail


USERNAME=$(kubectl exec -it vault-default-user-server-0 -c rabbitmq -- rabbitmqctl  list_users  --formatter=json | jq -r .[0].user)

if [ "$USERNAME" = "rabbitmq" ]; then
    echo "User existed"
else
    echo "User rabbitmq did not exist"
    exit 1
fi

