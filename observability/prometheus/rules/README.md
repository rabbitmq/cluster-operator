# Prometheus Rules

This directory contains Prometheus rules.
All rules are [alerting rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) except for [rabbitmq/recording-rules.yml](rabbitmq/recording-rules.yml) which contains [recording rules](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/).

Check the `spec` of your installed Prometheus custom resource.
In this example, let's assume your Prometheus spec contains the following fields:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
   ...
spec:
  ...
  ruleNamespaceSelector: {} # auto discover PrometheusRules accross all namespaces
  ruleSelector:
    matchLabels:
      release: my-prometheus
  ...
  version: v2.26.0
```

Given the `matchLabels` field from the Prometheus spec above, you would need to add the label `release: my-prometheus` to the `PrometheusRule` objects.
