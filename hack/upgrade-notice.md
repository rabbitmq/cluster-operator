⚠️ Upgrading the cluster-operator to this version will update RabbitMQ clusters (i.e. will cause rolling update of the underlying StatefulSets).
If you want to control when a RabbitMQ cluster gets updated, make sure to pause reconciliation before upgrading the cluster-operator.
After upgrading the cluster-operator, resume reconciliation whenever it's safe to update the RabbitMQ cluster.
See [Pause reconciliation for a RabbitMQ cluster](https://www.rabbitmq.com/kubernetes/operator/using-operator#pause).
