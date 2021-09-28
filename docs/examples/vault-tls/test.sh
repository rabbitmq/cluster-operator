#!/bin/bash
set -euo pipefail

assertTLS() {
  kubectl exec vault-tls-server-0 -c rabbitmq -- openssl s_client \
      -connect "$1" \
      -CAfile /etc/rabbitmq-tls/ca.crt \
      -verify_return_error
}

RABBITMQ_NAMESPACE=${RABBITMQ_NAMESPACE:-'examples'}

# Test TLS succeeds via client Service from server-0 to server-[0|1|2]
assertTLS "vault-tls.$RABBITMQ_NAMESPACE.svc.cluster.local:5671"
assertTLS "vault-tls.$RABBITMQ_NAMESPACE.svc.cluster.local:15671"

# Test TLS succeeds via headless Service from server-0 to server-2
assertTLS "vault-tls-server-2.vault-tls-nodes.$RABBITMQ_NAMESPACE:5671"
assertTLS "vault-tls-server-2.vault-tls-nodes.$RABBITMQ_NAMESPACE:15671"

# In this example, RabbitMQ gets only certs from Vault, but not the default user credentials.
# Therefore, check that the default user K8s Secret is present.
kubectl get secret vault-tls-default-user

helm uninstall vault
