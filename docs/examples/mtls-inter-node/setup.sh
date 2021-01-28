#!/bin/bash

OPENSSL=${OPENSSL:-openssl}

# Generate CA certificate and key
#
# These commands do not work with LibreSSL which is shipped with MacOS. Please use openssl
#
if $OPENSSL version | grep -q LibreSSL; then
  echo "Please do not use LibreSSL. Set OPENSSL variable to actual OpenSSL binary."
  exit 1
fi

$OPENSSL genrsa -out rabbitmq-ca-key.pem 2048
$OPENSSL req -x509 -new -nodes -key rabbitmq-ca-key.pem -subj "/CN=mtls-inter-node" -days 3650 -reqexts v3_req -extensions v3_ca -out rabbitmq-ca.pem

# Create a CA secret
kubectl create secret tls rabbitmq-ca --cert=rabbitmq-ca.pem --key=rabbitmq-ca-key.pem

# Create an Issuer (Cert Manager CA)
kubectl apply -f rabbitmq-ca.yaml

# Create a certificate for the cluster
kubectl apply -f rabbitmq-certificate.yaml

# Create a configuration file for Erlang Distribution
kubectl create configmap mtls-inter-node-tls-config --from-file=inter_node_tls.config

