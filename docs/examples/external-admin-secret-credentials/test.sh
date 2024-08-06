#!/bin/bash

set -eo pipefail

kubectl exec external-secret-user-server-0 -c rabbitmq -- \
  rabbitmqctl authenticate_user my-admin super-secure-password
