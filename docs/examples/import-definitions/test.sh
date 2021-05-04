#!/bin/bash

pushd "$(mktemp -d)" || exit 1

set -x
kubectl exec import-definitions-server-0 -c rabbitmq -- rabbitmqadmin \
  --format=raw_json --vhost=hello-world --username=hello-world \
  --password=hello-world --host=import-definitions.examples.svc \
  list queues &> queues.json

[[ "$(jq '.[0].name' queues.json)" == '"cq1"' ]] || exit 2
[[ "$(jq '.[1].name' queues.json)" == '"qq1"' ]] || exit 2

popd || exit 1

