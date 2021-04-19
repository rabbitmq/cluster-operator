# Prometheus Example

See [Monitoring RabbitMQ in Kubernetes](https://www.rabbitmq.com/kubernetes/operator/operator-monitoring.html) for detailed instructions.

If you deployed the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator), make Prometheus scrape RabbitMQ nodes by:
```shell
kubectl apply -f rabbitmq-servicemonitor.yaml
```

Make Prometheus scrape the RabbitMQ Cluster Operator by:
```shell
kubectl apply -f rabbitmq-cluster-operator-podmonitor.yaml
```
