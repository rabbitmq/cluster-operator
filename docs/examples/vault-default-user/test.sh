#!/bin/bash
set -x

# assert rabbitmq has a user with the same credentials defined in vault : rabbitmq:salsa
kubectl exec vault-default-user-server-0 -c rabbitmq -- rabbitmqctl list_users --no-table-headers | grep rabbitmq \
  || (echo "rabbitmq user does not exist"; exit -1)

