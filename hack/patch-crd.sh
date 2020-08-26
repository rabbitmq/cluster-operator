#!/bin/bash

kubectl patch -f config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml -p "$(cat hack/crd-patch.json)" --type json --local  -o yaml > /tmp/rabbitmq.com_rabbitmqclusters.yaml
mv /tmp/rabbitmq.com_rabbitmqclusters.yaml config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml
