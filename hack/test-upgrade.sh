#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
BASELINE_MANIFEST=$1
UPGRADE_MANIFEST=$2
OUTPUT_NOTICE=$3

touch $OUTPUT_NOTICE

cd $SCRIPT_DIR/..
make install-tools
make destroy
kubectl apply -f $BASELINE_MANIFEST
kubectl --namespace=rabbitmq-system wait --for=condition=Available deployment/rabbitmq-cluster-operator

cat <<EOF | kubectl apply -f -
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: operator-upgrade
  namespace: default
  labels:
    app: myrabbit
spec:
  image: rabbitmq:management
  persistence:
    storage: 20Gi
  replicas: 3
  resources:
    limits: {}
    requests: {}
  service:
    type: ClusterIP
EOF
kubectl wait rabbitmqcluster operator-upgrade --for=condition=AllReplicasReady --timeout=5m
kubectl wait rabbitmqcluster operator-upgrade --for=condition=ReconcileSuccess --timeout=5m

current_generation="$(kubectl --namespace default get sts operator-upgrade-server -ojsonpath='{.status.observedGeneration}')"

kubectl apply -f $UPGRADE_MANIFEST
kubectl --namespace=rabbitmq-system wait --for=condition=Available deployment/rabbitmq-cluster-operator
sleep 30
kubectl wait rabbitmqcluster operator-upgrade --for=condition=AllReplicasReady --timeout=5m
kubectl wait rabbitmqcluster operator-upgrade --for=condition=ReconcileSuccess --timeout=5m
upgrade_generation="$(kubectl --namespace default get sts operator-upgrade-server -ojsonpath='{.status.observedGeneration}')"

if (( current_generation != upgrade_generation ))
then
  cat $SCRIPT_DIR/upgrade-notice.md > $OUTPUT_NOTICE
fi

make destroy