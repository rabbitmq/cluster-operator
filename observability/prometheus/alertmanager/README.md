# Alertmanager

The file [slack.yml](./slack.yml) contains [Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/) configuration that sends Slack notifications for RabbitMQ and RabbitMQ Cluster Operator alerts.
It groups alerts by namespace and RabbitMQ cluster.

Replace
* `((ALERTMANAGER_NAME))` with the name of the Alertmanager CRD object
* `((SLACK_CHANNEL))` with the Slack channel name (e.g. `#my-channel`)
* `((SLACK_API_URL))` with the Slack Webhook URL (see [here](https://api.slack.com/messaging/webhooks) how to create one, e.g. `https://hooks.slack.com/services/paste/your/token`)

If the Secret already exists, either edit the existing secret or, if you want to define multiple Alertmanager configurations, use the custom resource [AlertmanagerConfig](https://github.com/prometheus-operator/prometheus-operator/blob/master/Documentation/user-guides/alerting.md#alertmanagerconfig-resource).
Deploy the Secret into the same namespace where Alertmanager is running.
