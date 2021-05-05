#!/bin/bash

set -ex
kubectl exec -it federation-server-0 -c rabbitmq -- rabbitmqadmin --username admin --password admin \
  --vhost=upstream publish exchange=example routing_key=123 payload="1234"

kubectl exec -it federation-server-0 -c rabbitmq -- rabbitmqadmin --username admin --password admin \
  --vhost=downstream --format=pretty_json get queue=qq1 ackmode='ack_requeue_false' \
  | jq -e '.[].payload'

kubectl exec -it federation-server-0 -c rabbitmq -- rabbitmqadmin --username admin --password admin \
  --vhost=downstream --format=pretty_json get queue=cq1 ackmode='ack_requeue_false' \
  | jq -e '.[].payload'

