#!/usr/bin/env bash

set -e

fly -t rmq set-pipeline -p operator -c pipeline.yml \
  -v git-ssh-key="$(lpass show "Shared-RabbitMQ for Kubernetes/rmq-k8s-ci-git-ssh-key" --note)" \
  -v gcr-push-service-account-key="$(lpass show "Shared-RabbitMQ for Kubernetes/rmq-k8s-ci-gcr-push" --note)" \
  -v rmq-k8s-ci-kubectl-service-account-key="$(lpass show "Shared-RabbitMQ for Kubernetes/rmq-k8s-ci-kubectl-service-account-key" --note)"
