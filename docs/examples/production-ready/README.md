# Production Example

This is an example of a good starting point for a production RabbitMQ deployment.
It deploys a 3-node cluster with sufficient resources to handle 1 billion messages per day at 8kB payload and a replication factor of three.
The rest of the workload details are outlined in the monthly cost savings calculator on https://rabbitmq.com/tanzu

Please keep in mind that:

1. It may not be suitable for **your** production deployment.
   The official [RabbitMQ Production Checklist](https://www.rabbitmq.com/production-checklist.html) will help you with some of these considerations.

2. While it is important to correctly deploy RabbitMQ cluster for production workloads, it is equally important for your applications to use RabbitMQ correctly.
   [Production Checklist](https://www.rabbitmq.com/production-checklist.html) covers some of the common issues such as connection churn and polling consumers.
   This example was tested with [Quorum Queues](https://www.rabbitmq.com/quorum-queues.html) which provide excellent data safety for workloads that require message replication.

Before you can deploy this RabbitMQ cluster, you will need a multi-zone Kubernetes cluster with at least 3 nodes, 12 CPUs, 30Gi RAM and 1.5Ti disk space available.
A `storageClass` named `ssd` will need to be defined too.
We have [a GKE-specific example](ssd-gke.yaml) included in this example.
Read more about the expected disk performance [in Google Cloud Documentation](https://cloud.google.com/compute/docs/disks/performance#ssd_persistent_disk).
For what it's worth, disk write throughput is the limiting factor for persistent messages with a payload of 8kB.

To deploy this RabbitMQ cluster, run the following:

```shell
kubectl apply -f rabbitmq.yaml
kubectl apply -f pod-disruption-budget.yaml
```

## Q & A

### Is 4 CPUs per RabbitMQ node the minimum?

No. The absolute minimum is 2 CPUs.

For our workload - 1 billion messages per day at 8kB payload and a replication factor of three - 4 CPUs is the minimum.

### Will RabbitMQ work with 1 CPU?

Yes. It will work, but poorly, which is why we cannot recommend it for production workloads.
A RabbitMQ with less than 2 full CPUs cannot be considered production.


### Can I assign less than 1 CPU to RabbitMQ?

Yes, this is entirely possible within Kubernetes.
Be prepared for unresponsiveness that cannot be explained.
The kernel will work against RabbitMQ's runtime optimisations, and anything can happen.
A RabbitMQ with less than 2 full CPUs cannot be considered production.

### Does CPU clock speed matter for message throughput?

Yes. Queues are single threaded, and CPUs with higher clock speeds can run more cycles, which means that the queue process can perform more operations per second.
This will not the case when disks or network are the limiting factor, but in benchmarks with sufficient network and disk capacity, faster CPUs translate to higher message throughhput.

### Are vCPUs (virtual CPUs) OK?

Yes. The workload that was used for this production configuration starting point ran on Google Cloud and used 2 real CPU cores with 2 hyper-threads each, meaning 4 vCPUs.
While we would recommend real CPUs and no hyper-threading, we also operate in the cloud and default to using vCPUs, including for our benchmarks.
