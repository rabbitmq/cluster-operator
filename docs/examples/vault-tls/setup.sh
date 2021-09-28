#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "$SCRIPT_DIR"/../vault-default-user/setup.sh

echo "Configuring PKI engine..."
# Required so that each RabbitmMQ Pod can generate its own short-lived TLS server certificate.
vault_exec "vault secrets enable pki"
vault_exec "vault write pki/root/generate/internal common_name=my-website.com ttl=8760h"
vault_exec "vault write pki/config/urls issuing_certificates=\"http://127.0.0.1:8200/v1/pki/ca\""
vault_exec "vault write pki/roles/cert-issuer allowed_domains=${RABBITMQ_NAMESPACE},${RABBITMQ_NAMESPACE}.svc allow_subdomains=true max_ttl=1h"

# Modify the policy to create RabbitMQ server leaf certificates.
# The path need to be referenced from the RabbitmqCluster CRD spec.secretBackend.vault.tls.pkiIssuerPath
vault_exec "vault policy write rabbitmq-policy - <<EOF
path \"pki/issue/cert-issuer\" {
    capabilities = [\"create\", \"update\"]
}
EOF
"
