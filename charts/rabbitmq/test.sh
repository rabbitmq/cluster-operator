#!/bin/bash

# RabbitMQ Cluster Operator
#
# Copyright 2020 VMware, Inc. All Rights Reserved.
#
# This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
#
# This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

set -e
set -o pipefail

if ! yq --version | grep -q 'yq version 3'; then
  echo "Please install https://github.com/mikefarah/yq v3"
  exit 1
fi

for p in $(kubectl eg rabbitmqclusters.rabbitmq.com | yq d - spec.resources |  yq d - spec.tolerations |  yq d - spec.override | yq d - spec.affinity | yq r - spec | grep -v ' - ' | awk -F: '{ print $1 }'); do
    grep -q "$p " templates/rabbitmq.yaml
    if [[ $? != 0 ]]; then
      echo "FAIL: Property $p not exposed in the helm chart"
      exit 1
    fi
  done
echo "Seems like all CRD properties are exposed in the helm chart"

chart=$(helm package . | helm package . | awk '{print $NF}')

helm template $chart -f example-configurations.yaml > template-output

# it should be updated if we add any new configurations and when we modify plans/example-configurations.yaml
diff -u template-output expected-template-output && echo "Successfully rendered the template"
