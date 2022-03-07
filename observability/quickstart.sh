#!/usr/bin/env bash

# Prerequisites:
# * kubectl 1.18+ points to a running K8s cluster
# * helm 3+
# * environment variables SLACK_CHANNEL and SLACK_API_URL are set

set -eo pipefail
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

GREEN='\033[0;32m'
ORANGE='\033[0;33m'
NO_COLOR='\033[0m'

printf "%bInstalling kube-prometheus-stack...%b\n" "$GREEN" "$NO_COLOR"
# helm search repo prometheus-community/kube-prometheus-stack
KUBE_PROMETHEUS_STACK_VERSION='31.0.2'
KUBE_PROMETHEUS_STACK_NAME='prom'
KUBE_PROMETHEUS_STACK_NAMESPACE='kube-prometheus'
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade "$KUBE_PROMETHEUS_STACK_NAME" prometheus-community/kube-prometheus-stack \
  --version "$KUBE_PROMETHEUS_STACK_VERSION" \
  --install \
  --namespace "$KUBE_PROMETHEUS_STACK_NAMESPACE" \
  --create-namespace \
  --wait \
  --set "defaultRules.create=false" \
  --set "nodeExporter.enabled=false" \
  --set "prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues=false" \
  --set "prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false" \
  --set "prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false" \
  --set "prometheus.prometheusSpec.probeSelectorNilUsesHelmValues=false" \
  --set "alertmanager.alertmanagerSpec.useExistingSecret=true" \
  --set "grafana.env.GF_INSTALL_PLUGINS=flant-statusmap-panel" \
  --set "grafana.adminPassword=admin"

printf "%bInstalling ServiceMonitor and PodMonitor..%b\n" "$GREEN" "$NO_COLOR"
kubectl -n "$KUBE_PROMETHEUS_STACK_NAMESPACE" apply --filename "$DIR"/prometheus/monitors/

printf "%bInstalling Prometheus rules...%b\n" "$GREEN" "$NO_COLOR"
kubectl -n "$KUBE_PROMETHEUS_STACK_NAMESPACE" apply --recursive --filename "$DIR"/prometheus/rules/

printf "%bInstalling Alertmanager Slack configuration...%b\n" "$GREEN" "$NO_COLOR"
kubectl -n "$KUBE_PROMETHEUS_STACK_NAMESPACE" apply --filename <(sed \
  -e "s/((ALERTMANAGER_NAME))/${KUBE_PROMETHEUS_STACK_NAME}-kube-prometheus-stack-alertmanager/" \
  -e "s/((SLACK_CHANNEL))/${SLACK_CHANNEL}/" \
  -e "s|((SLACK_API_URL))|${SLACK_API_URL}|" \
  "$DIR"/prometheus/alertmanager/slack.yml)

if [[ -z "$SLACK_CHANNEL" || -z "$SLACK_API_URL" ]]
then
  printf "%bEnvironment variables SLACK_CHANNEL or SLACK_API_URL not set. " "$ORANGE"
  printf "You will therefore not receive Slack notifications.%b\n" "$NO_COLOR"
fi

printf "%bInstalling RabbitMQ Grafana Dashboards...%b\n" "$GREEN" "$NO_COLOR"
kubectl -n "$KUBE_PROMETHEUS_STACK_NAMESPACE" apply --filename "$DIR"/grafana/dashboards/

printf "%bInstalling RabbitMQ Cluster Operator...%b\n" "$GREEN" "$NO_COLOR"
kubectl apply --filename https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml

printf "\n%bTo open Prometheus UI execute \nkubectl -n %s port-forward svc/%s-kube-prometheus-stack-prometheus 9090\nand open your browser at http://localhost:9090\n\n" "$GREEN" "$KUBE_PROMETHEUS_STACK_NAMESPACE" "$KUBE_PROMETHEUS_STACK_NAME"
printf "To open Alertmanager UI execute \nkubectl -n %s port-forward svc/%s-kube-prometheus-stack-alertmanager 9093\nand open your browser at http://localhost:9093\n\n" "$KUBE_PROMETHEUS_STACK_NAMESPACE" "$KUBE_PROMETHEUS_STACK_NAME"
printf "To open Grafana UI execute \nkubectl -n %s port-forward svc/%s-grafana 3000:80\nand open your browser at http://localhost:3000\nusername: admin, password: admin%b\n" "$KUBE_PROMETHEUS_STACK_NAMESPACE" "$KUBE_PROMETHEUS_STACK_NAME" "$NO_COLOR"
