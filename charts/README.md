# RabbitMQ Cluster Operator Helm charts

This folder contains Helm charts to deploy our components:

* `operator` chart deploys RabbitMQ Cluster Operator and the Custom Resource Definition (CRD)
* `rabbitmq` chart deploys a `RabbitmqCluster` resource

`rabbitmq` chart can also be used in combination with [Tanzu Services Manager](https://docs.pivotal.io/ksm/) and the `tsmgr` direcotry for OSBAPI integration.

Please refer to the [rabbitmq.com/install-cluster-operator.html](https://www.rabbitmq.com/install-cluster-operator.html) to install RabbitMQ Cluster Operator using these charts.
