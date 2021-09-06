#!/bin/bash
set -xeo pipefail

# give a user exists with password a

# then when we change the password to be b

# assert that the passowrd has changed in rabbitmq

# assert rabbitmq has a user with the same credentials defined in vault : rabbitmq:salsa
USERNAME=$(kubectl exec -it vault-default-user-server-0 -c rabbitmq -- rabbitmqctl  list_users  --formatter=json | jq -r .[0].user)

if [ "$USERNAME" = "rabbitmq" ]; then
    echo "User existed"
else
    echo "User rabbitmq did not exist"
    exit 1
fi

