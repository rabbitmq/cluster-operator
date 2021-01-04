# Prometheus Example

See [Monitoring RabbitMQ in Kubernetes](https://www.rabbitmq.com/kubernetes/operator/operator-monitoring.html) for detailed instructions.

If you deployed the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator), make Prometheus scrape RabbitMQ nodes by:
```shell
kubectl apply -f rabbitmq-podmonitor.yaml
```

Alternatively, if you deployed the Prometheus Operator via the [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) Helm chart,
set the values in [kube-prometheus-stack-values.yaml.example](kube-prometheus-stack-values.yaml.example) when installing / upgrading the Helm chart.

---
## TLS Endpoints

With `TLS` enabled you should use `-tls` files to deploy the secure prometheus endpoints. 
_Note_: The standard Prometheus (15692) port is disabled with the option `disableNonTLSListeners=true`.  