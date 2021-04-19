# RabbitMQ Prometheus Rules

## Adding new Prometheus Rules

To allow filtering in the [RabbitMQ-Alerts Grafana dashboard](../../../grafana/dashboards/rabbitmq-alerts.yml) and allowing [configuration of Alertmanager](../../alertmanager) all RabbitMQ alerts must output at least the labels:
* `rulesgroup: rabbitmq`
* `namespace`
* `rabbitmq_cluster`
* `severity`

All alerts should output the annotations:
* `description`: technical description with interpolated value from PromQL
* `summary`: meaning of the alert written in a format that is easy to understand by humans, preferrably with a run book and links to more detailed documentation
