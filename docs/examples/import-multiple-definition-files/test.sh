#!/bin/bash

set -x

kubectl exec import-multiple-definition-files-server-0 -c rabbitmq -- rabbitmqctl authenticate_user guest guest
