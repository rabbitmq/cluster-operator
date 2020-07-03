#!/bin/bash

kubectl apply -f upstream.yaml
kubectl apply -f downstream.yaml

sleep 2

kubectl wait --for=condition=Ready pod/upstream-rabbitmq-server-0
kubectl wait --for=condition=Ready pod/downstream-rabbitmq-server-0

UPSTREAM_USERNAME=$(kubectl get secret upstream-rabbitmq-admin -o jsonpath="{.data.username}" | base64 --decode)
UPSTREAM_PASSWORD=$(kubectl get secret upstream-rabbitmq-admin -o jsonpath="{.data.password}" | base64 --decode)
DOWNSTREAM_USERNAME=$(kubectl get secret downstream-rabbitmq-admin -o jsonpath="{.data.username}" | base64 --decode)
DOWNSTREAM_PASSWORD=$(kubectl get secret downstream-rabbitmq-admin -o jsonpath="{.data.password}" | base64 --decode)

kubectl exec downstream-rabbitmq-server-0 -- rabbitmqctl set_parameter federation-upstream my-upstream "{\"uri\":\"amqps://${UPSTREAM_USERNAME}:${UPSTREAM_PASSWORD}@upstream-rabbitmq-client\",\"expires\":3600001}"

kubectl exec downstream-rabbitmq-server-0 -- rabbitmqctl set_policy --apply-to exchanges federate-me "^amq\." '{"federation-upstream-set":"all"}'

echo "**********************************************************"
echo "* PLEASE RUN 'sudo kubefwd svc' TO START PORT FORWARDING *"
echo "*              and press ENTER when ready                *"
echo "**********************************************************"
read

UPSTREAM_RABBITMQADMIN="rabbitmqadmin -U http://upstream-rabbitmq-client/ -u ${UPSTREAM_USERNAME} -p ${UPSTREAM_PASSWORD} -V /"
DOWNSTREAM_RABBITMQADMIN="rabbitmqadmin -U http://downstream-rabbitmq-client/ -u ${DOWNSTREAM_USERNAME} -p ${DOWNSTREAM_PASSWORD} -V /"

$RABBITMQADMIN_UPSTREAM declare queue name=test.queue queue_type=quorum
$RABBITMQADMIN_UPSTREAM declare binding source=amq.fanout destination=test.queue

$DOWNSTREAM_RABBITMQADMIN declare queue name=test.queue queue_type=quorum
$DOWNSTREAM_RABBITMQADMIN declare binding source=amq.fanout destination=test.queue

$UPSTREAM_RABBITMQADMIN publish exchange=amq.fanout routing_key=test payload="hello, world"
$DOWNSTREAM_RABBITMQADMIN get queue=test.queue ackmode=ack_requeue_false
