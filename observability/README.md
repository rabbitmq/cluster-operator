# Observability

This directory contains scrape targets, RabbitMQ alerting rules, Alertmanager configuration, and RabbitMQ dashboards.

## Quick Start

If you don't have Prometheus and Grafana installed, the quickest way to try out RabbitMQ observability is as follows:

Make sure
1. `kubectl` (version 1.18+) pointing to any running Kubernetes cluster
1. `helm` (version 3+) being installed

Optionally, if you want to receive Slack notifications for RabbitMQ alerts, you will need a Slack Webhook URL (see [here](https://api.slack.com/messaging/webhooks) how to create one).

```bash
# Optionally, to receive Slack notifications on RabbitMQ alerts, set the name of the Slack channel and the Slack Webhook URL:
export SLACK_CHANNEL='#my-channel'
export SLACK_API_URL='https://hooks.slack.com/services/paste/your/token'

./quickstart.sh
```

This will install Prometheus Operator, Prometheus, kube-state-metrics, Alertmanager, Grafana and will set up RabbitMQ scrape targets, RabbitMQ alerting rules, Slack notifications, and RabbitMQ Grafana dashboards.
Note that the [quickstart.sh](./quickstart.sh) script is not a production-ready setup. Refer to the official Prometheus and Grafana documentation on how to deploy a production-ready monitoring stack.

Learn more on RabbitMQ monitoring in:
* [RabbitMQ Prometheus documentation](https://www.rabbitmq.com/prometheus.html)
* [Operator monitoring documentation](https://www.rabbitmq.com/kubernetes/operator/operator-monitoring.html)
* [TGIR S01E07: How to monitor RabbitMQ?](https://youtu.be/NWISW6AwpOE)
* [Notify me when RabbitMQ has a problem](https://blog.rabbitmq.com/posts/2021/05/alerting/)
