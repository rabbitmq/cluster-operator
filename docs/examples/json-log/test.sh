#!/bin/bash

set -eo pipefail

kubectl logs json-server-0 -c rabbitmq --tail=3 | jq .
