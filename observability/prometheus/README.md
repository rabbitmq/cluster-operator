# Prometheus configuration

RabbitMQ alerting rules depend on [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics). Refer to the kube-state-metrics documentation to deploy and scrape kube-state-metrics.

## With Prometheus Operator
If Prometheus and Alertmanager are installed by [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator), apply the YAML files in [alertmanager](./alertmanager), [monitors](./monitors), and [rules](./rules) directories. They contain K8s objects watched by Prometheus Operator configuring Prometheus.

## Without Prometheus Operator
If Prometheus and Alertmanager are not installed by Prometheus Operator, use [config-file.yml](./config-file.yml) and [rule-file.yml](./rule-file.yml) as a starting point for RabbitMQ monitoring and alerting.
`rule-file.yml` is an auto-generated file containing the same rules as the [rules](./rules/) directory.

For the [Alertmanager configuration file](https://prometheus.io/docs/alerting/latest/configuration/#configuration-file), use the same `alertmanager.yaml` as provided in [alertmanager/slack.yml](alertmanager/slack.yml).
