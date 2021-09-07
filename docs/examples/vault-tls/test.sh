#!/bin/bash
set -xeo pipefail


assertTLS() {
  kubectl exec vault-tls-server-0 -c rabbitmq -- openssl s_client -connect vault-tls.example.svc.cluster.local:"$1" -verify_return_error -CAfile /etc/rabbitmq-tls/ca.crt
}

assertTLS 5671
assertTLS 15671
