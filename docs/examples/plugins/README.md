# Plugins Example

You can enable RabbitMQ plugins by setting `.spec.rabbitmq.additionalPlugins`. It is a list value that will be appended to the list of plugins enabled by the operator that are considered essential and therefore always enabled.

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```

And once deployed, you can check the list of enabled plugins like this:

```shell
kubectl get -o yaml configmap plugins-rabbitmq-server-conf
```

Changes to `additionalPlugins` do not require cluster restart. If you edit this field, Cluster Operator will run `rabbitmq-plugins` inside the running containers and enable/disable plugins without restarting pods.