#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

pushd $SCRIPT_DIR/../..
make cert-manager
popd

cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
EOF

## Removing stale namespace. Most likely due to previous test failures
kubectl delete namespaces examples --ignore-not-found --timeout=5m
for example in $(find $SCRIPT_DIR -mindepth 1 -type d)
do
  [[ -e "$example"/.ci-skip ]] && continue
  pushd "$example"
  kubectl create namespace examples
  kubectl config set-context --current --namespace=examples
  [[ -e setup.sh ]] && ./setup.sh
  kubectl apply -f rabbitmq.yaml --timeout=30s
  kubectl wait -f rabbitmq.yaml --for=condition=AllReplicasReady --timeout=5m
  kubectl wait -f rabbitmq.yaml --for=condition=ReconcileSuccess --timeout=5m
  ./test.sh
  ## Teardown
  kubectl delete -f rabbitmq.yaml --timeout=3m
  kubectl delete namespace examples --timeout=10m
  popd
done
