# Network Policy Example

Kubernetes allows you to restrict the source/destination of traffic to & from your Pods at an IP / port level, by defining NetworkPolicies for your cluster, provided your
cluster has the [network plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/) enabled. For more
information on NetworkPolicies, see the [Kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/).

By defining NetworkPolicies, you can restrict the network entities with which your RabbitmqCluster can communicate, and prevent unrecognised traffic
from reaching the cluster. It is important to note that once a RabbitmqCluster Pod, or any other Pod for that matter, is the target of any
NetworkPolicy, it becomes isolated to all traffic except that permitted by a NetworkPolicy.

The first policy in this example, [allow-inter-node-traffic.yaml](./allow-inter-node-traffic.yaml) ensures that the only nodes in the RabbitmqCluster
send or receive traffic with each other on the ports used for inter-node communication.

The second policy, [allow-operator-traffic.yaml](./allow-operator-traffic.yaml), allows the cluster-operator and the messaging-topology-operator to
communicate with the cluster Pods over HTTP, which is necessary for some reconciliation operations.

The third policy, [allow-rabbitmq-traffic.yaml](./allow-rabbitmq-traffic.yaml), allows all ingress traffic to external-facing ports on the cluster,
such as for AMQP messaging, Prometheus scraping, etc. In practice you may wish to add a selector to this policy to only allow traffic to these
ports from your known client application Pods, or Prometheus servers, etc., depending on your network topology.

The ports exposed in these examples are taken from [the RabbitMQ documentation](https://www.rabbitmq.com/networking.html#ports), and represent
the default ports used by RabbitMQ. It is possible to configure different ports to be used; if you have applied such configuration in your cluster,
take care to ensure the ports in your NetworkPolicies match this configuration.
