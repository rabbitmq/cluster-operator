# Production Example

Before you can deploy this RabbitMQ cluster, you will need a multi-zone Kubernetes deployment with at least 3 worker nodes, each in a different zone, and each with 4 CPUs and 10Gi RAM available for RabbitMQ.

A `storageClass` named `ssd` will need to be defined too.
Feel free to use the [GKE-specific example](ssd-gke.yaml) included in this example for reference.
Each RabbitMQ node will provision a 500Gi persistent volume of type `ssd`.
This configuration is a requirement for sustaining 1 billion persistent messages per day of 8kB payload each and a replication factor of three using [quorum queues](https://www.rabbitmq.com/quorum-queues.html).

To deploy this RabbitMQ cluster, run the following:

```shell
kubectl apply -f rabbitmq.yaml
kubectl apply -f pod-disruption-budget.yaml
```

This example is a good starting point for a production RabbitMQ deployment and it may not be suitable for **your use-case**.
This RabbitMQ cluster can sustain 1 billion persistent messages per day at 8kB payload and a replication factor of three using [quorum queues](https://www.rabbitmq.com/quorum-queues.html).
The rest of the workload details are outlined in this [monthly cost savings calculator](https://rabbitmq.com/tanzu#calculator).

While a RabbitMQ cluster with sufficient resources is important for production, it is equally important for your applications to use RabbitMQ correctly.
Applications that open & close connections frequently, polling consumers and consuming one message at a time are common issues that make RabbitMQ "slow".

The official [Production Checklist](https://www.rabbitmq.com/production-checklist.html) will help you optimise RabbitMQ for your use-case.



## Q & A


### Are 4 CPUs per RabbitMQ node a minimum requirement for production?

No. The absolute minimum is 2 CPUs.

Our workload - 1 billion persistent messages per day of 8kB payload and a replication factor of three - requires 4 CPUs.


### Will RabbitMQ work with 1 CPU?

Yes. It will work, but poorly, which is why we cannot recommend it for production workloads.

A RabbitMQ with less than 2 full CPUs cannot be considered "production".


### Can I assign less than 1 CPU to RabbitMQ?

Yes, this is entirely possible within Kubernetes.
Be prepared for unresponsiveness that cannot be explained.

A RabbitMQ with less than 2 full CPUs cannot be considered "production".


### Does CPU clock speed matter for message throughput?

Yes. Queues are single threaded, and CPUs with higher clock speeds can run more cycles, which means that a queue process can perform more operations per second.
This will not be the case when disks or network are the limiting factor, but in benchmarks with sufficient network and disk capacity, faster CPUs usually translate to higher message throughhput.


### Are vCPUs (virtual CPUs) OK?

Yes. The workload that was used for this production configuration ran on Google Cloud and used 2 real CPU cores with 2 hyper-threads each, meaning 4 vCPUs.
While we recommend real CPUs and no hyper-threading, we also operate in the cloud and default to using vCPUs, including for our benchmarks.
