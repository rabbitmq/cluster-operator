# Prometheus Scrape Targets

This directory contains Prometheus scrape targets.

Check the `spec` of your installed Prometheus custom resource.
In this example, let's assume your Prometheus spec contains the following fields:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  ...
  podMonitorNamespaceSelector: {}
  podMonitorSelector:
    matchLabels:
      release: my-prometheus
  ...
  serviceMonitorNamespaceSelector: {}
  serviceMonitorSelector:
    matchLabels:
      release: my-prometheus
  ...
  version: v2.26.0
```

Given the `matchLabels` fields from the Prometheus spec above, you would need to add the label `release: my-prometheus` to the `PodMonitor` and `ServiceMonitor` objects.
Since the `NamespaceSelector` is empty, deploy the objects into the same namespace where Prometheus is running.

File [rabbitmq-servicemonitor.yml](./rabbitmq-servicemonitor.yml) contains scrape targets for RabbitMQ.
Metrics listed in [RabbitMQ metrics](https://github.com/rabbitmq/rabbitmq-server/blob/master/deps/rabbitmq_prometheus/metrics.md) will be scraped from all RabbitMQ nodes.
Note that the ServiceMonitor object works only for RabbitMQ clusters deployed by [cluster-operator](https://github.com/rabbitmq/cluster-operator) `>v1.6.0`. If you run cluster-operator `<=v1.6.0` use a PodMonitor instead:

```yaml
---
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: rabbitmq
spec:
  podMetricsEndpoints:
  - port: prometheus
    interval: 15s
  selector:
    matchLabels:
      app.kubernetes.io/component: rabbitmq-operator
  namespaceSelector:
    any: true
```

File [rabbitmq-cluster-operator-podmonitor.yml](./rabbitmq-cluster-operator-podmonitor.yml) contains a scrape target for the RabbitMQ Cluster Operator.
[The metrics](https://book.kubebuilder.io/reference/metrics.html) emitted by the RabbitMQ Cluster Operator are created by Kubernetes controller-runtime and are therefore completely different from the RabbitMQ metrics.
