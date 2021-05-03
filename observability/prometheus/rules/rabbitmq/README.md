# RabbitMQ Prometheus Rules

This directory splits Prometheus rules into different files so that you can apply rules individually.
Although the [rule groups](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/#rule_group) in this directory have the same name `rabbitmq`, they are in fact different rule groups evaluated at different points in time without clear evaluation order.

## Adding new Prometheus Rules

To allow filtering in the [RabbitMQ-Alerts Grafana dashboard](../../../grafana/dashboards/rabbitmq-alerts.yml) and allowing [configuration of Alertmanager](../../alertmanager) all RabbitMQ alerts must output at least the labels:
* `rulesgroup: rabbitmq`
* `namespace`
* `rabbitmq_cluster`
* `severity`

Note that in some rules, the labels `namespace` and `rabbitmq_cluster` are implicitly output by the PromQL expression.
If these labels are not output by the PromQL expression, they must be added by the `labels` field.

All alerts should output the annotations:
* `description`: technical description with interpolated value from PromQL
* `summary`: meaning of the alert written in a format that is easy to understand by humans, preferrably with a run book and links to more detailed documentation
