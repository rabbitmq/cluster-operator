#!/bin/bash
set -euxo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# shellcheck source=../../../scripts/with_retry.sh
source "$SCRIPT_DIR/../../../scripts/with_retry.sh"

assertTLS() {
  kubectl exec tls-server-0 -c rabbitmq -- openssl s_client \
      -connect "$1" \
      -CAfile /etc/rabbitmq-tls/ca.crt \
      -verify_return_error \
      </dev/null
}

RABBITMQ_NAMESPACE=${RABBITMQ_NAMESPACE:-'examples'}

with_retry 'assertTLS "tls.$RABBITMQ_NAMESPACE.svc.cluster.local:5671"'
with_retry 'assertTLS "tls.$RABBITMQ_NAMESPACE.svc.cluster.local:15671"'
