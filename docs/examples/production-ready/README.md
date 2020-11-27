# Production Example

This is an example of a good starting point for a production RabbitMQ deployment. It deploys a 3-node cluster with enough resources to handle reasonable traffic.

Please keep in mind that:

1. It may not be suitable for YOUR production deployment. Please go through the [Production Checklist](https://www.rabbitmq.com/production-checklist.html) to learn more about production deployment considerations.

2. While it is important to correctly deploy RabbitMQ cluster for production deployment, it is even more important to correctly use RabbitMQ from your applications. [Production Checklist](https://www.rabbitmq.com/production-checklist.html) covers some of the common issues such as connection churn and polling cunsumers. Please also consider using [Quorum Queues](https://www.rabbitmq.com/quorum-queues.html) since they provide better data safety.

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```

Please keep in mind that you need a multi-zone Kubernetes cluster with 12 CPUs, 30Gi RAM, 1.5Ti disk space available as well as a `storageClass` called `ssd` to deploy this example as-is. Of course you can adjust these values to your environment if needed.

An SSD storage class can be defined using [the example](ssd-gke.yaml) (which is GKE-specific and needs to be adjusted for other environments). Read more about the expected disk performance [in Google Cloud Documentation](https://cloud.google.com/compute/docs/disks/performance#ssd_persistent_disk).
