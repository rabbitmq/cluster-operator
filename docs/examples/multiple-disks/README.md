# Multiple Disks Example

You can request and mount additional volumes using `.spec.override` feature and then configure RabbitMQ to use these volumes to store data. In this example we define two additional volumes:
1. `quorum-wal` for [quorum queue write-ahead log](https://www.rabbitmq.com/quorum-queues.html#resource-use)
1. `quorum-segments` for quorum queue segment files

You can read more about using multiple disks/volumes with RabbitMQ and why you may want to do that in our [Quorum queues and why disks matter](https://www.rabbitmq.com/blog/2020/04/21/quorum-queues-and-why-disks-matter/) blog post.

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```

And once deployed, you can check that RabbitMQ created files in the configured locations like this:

```shell
kubectl exec multiple-disks-rabbitmq-server-0 -- ls /var/lib/rabbitmq/quorum-wal /var/lib/rabbitmq/quorum-segments
```
