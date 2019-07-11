#!/usr/bin/env bash

set -e

fly -t rmq set-pipeline -p operator -c pipeline.yml \
  -v git-ssh-key="$(lpassd show "Shared-RabbitMQ for Kubernetes/rmq-k8s-ci-git-ssh-key" --note)" \
  -v gcr-push-service-account-key="$(lpassd show "Shared-RabbitMQ for Kubernetes/ci-gcr-access" --note | jq -c .)" \
  -v gcr-pull-service-account-key="$(lpassd show "Shared-RabbitMQ for Kubernetes/ci-gcr-pull" --note | jq -c .)" \
  -v rmq-k8s-ci-kubectl-service-account-key="$(lpassd show "Shared-RabbitMQ for Kubernetes/rmq-k8s-ci-kubectl-service-account-key" --note | jq -c .)" \
  -v pivnet-api-token="$(lpassd show "Shared-RabbitMQ for Kubernetes/pivnet-api-token" --password)" \
  -v pivnet-aws-access-key-id="$(lpassd show "Shared-RabbitMQ for Kubernetes/rabbitmq-for-kubernetes-pivnet-s3-user" --username)" \
  -v pivnet-aws-secret-access-key="$(lpassd show "Shared-RabbitMQ for Kubernetes/rabbitmq-for-kubernetes-pivnet-s3-user" --password)" \
  -v release-metadata-content="$(lpassd show "Shared-RabbitMQ for Kubernetes/pivnet-release-metadata" --notes)" \
  -v toolsmith-api-token="$(lpassd show 'Shared-RabbitMQ for Kubernetes/toolsmiths-api-token' --password)"
