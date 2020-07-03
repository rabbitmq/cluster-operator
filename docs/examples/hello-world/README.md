# Hello World Example

This is the simplest `RabbitmqCluster` definition. The only explicitly specified property is the name of the cluster. Everything else will be configured according to the Cluster Operator's defaults.

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```

And once deployed, you can check what defaults were applied like this (`spec` section is the most important):

```shell
kubectl get -o yaml rabbitmqclusters.rabbitmq.com hello-world
```