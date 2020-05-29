Gracefully terminating RabbitMQ pods
----

Background
===

RabbitMQ Cluster Operator uses StatefulSets and Persistent Volumes to maintain the state of the cluster in the event of a RabbitMQ node restart. Normally, **when** a container is allowed to restart is not a major concern in a Kubernetes cluster. The Kubelet on each node is responsible for managing Pods' containers and has free reign to restart them. In fact abstracting away container lifecycle management is one of the central aims of Kubernetes. This does not pose much of an issue for stateless workloads. Requests will load balance to other replicas (if they exist) and retries of in flight requests can be trivially configured at the app layer. However, this model poses a problem for rolling upgrades of RabbitMQ clusters.

Problem
====
We cannot attempt to provide zero-downtime rolling upgrades without tying Kubernetes lifecycle management to RabbitMQ app state.

The aim of a rolling upgrade is to change software with no or minimal service degradation. HA RabbitMQ (quorum or classic queues) replicates queue data to other nodes in the cluster to maintain consistency and availability if the node holding the leader fails. During a StatefulSet rolling upgrade, Pods are deleted in descending cardinal order. The next Pod is marked `Terminating` and reaped as soon as the previously restarted Pod is `Ready`.

For quorum queues with high throughput, triggering a rolling upgrade will lead to downtime. In experiments, we also observed a loss of availability once a quorum of nodes (**N/2 + 1**) had been restarted. Quorum queue members don't need to be completely synced to rejoin, but they will pause before rejoining to catch up if they are too far behind. In this interval, the remaining nodes of the cluster cannot elect a new leader and serve traffic.

In classic queues the replication strategy will dictate the consequence of not waiting for mirrors to be synced before rolling the next node: either there will be downtime waiting for synced replicas, or an unsynced leader will be elected and data will be lost.

Approach
===

### Checking for critical queues

The RabbitMQ core team [have developed](https://github.com/rabbitmq/rabbitmq-cli/issues/389) `rabbitmq-queues` CLI commands to check whether restarting a node would negatively impact its queues:

- [`rabbitmq-queues check_if_node_is_quorum_critical`](https://www.rabbitmq.com/rabbitmq-queues.8.html#check_if_node_is_quorum_critical): Health check that exits with a non-zero code if there are queues with minimum online quorum (queues that would lose their quorum if the target node is shut down).
- [`rabbitmq-queues check_if_node_is_mirror_sync_critical`](https://www.rabbitmq.com/rabbitmq-queues.8.html#check_if_node_is_mirror_sync_critical): Health check that exits with a non-zero code if there are classic mirrored queues without online synchronised mirrors (queues that would potentially lose data if the target node is shut down).

We have leveraged these checks in the [PreStop lifecycle hook](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks) of the RabbitMQ container on each Pod in the cluster. When a Pod enters `Terminating`, the Kubelet first executes the PreStop hook before killing the container. The hook executes synchronously, so we have to block in a loop until the checks pass. To avoid unrelated errors causing an infinite loop, we are explicit about which errors codes are blocking:

```
while true; do
  rabbitmq-queues check_if_node_is_quorum_critical 2>&1
  if [ $(echo $?) -eq 69 ]
    then sleep 2
    continue
  fi
  rabbitmq-queues check_if_node_is_mirror_sync_critical 2>&1
  if [ $(echo $?) -eq 69 ]
    then sleep 2
    continue
  fi

  break
done
```

We run the check in bash because we want the operator to work with upstream RabbitMQ images. We currently use the official RabbitMQ [Docker image](https://hub.docker.com/_/rabbitmq).

### Deleting the Custom Resource

When deleting the RabbitMQ Custom Resource, we don't want to have to wait on anything. We assume that if the user chooses to run `kubectl delete rabbitmqclusers my-cluster`, then they don't care about queue sync. We also don't want a situation where a final node can't be deleted because the check concludes the obvious but irrelevant fact that you will lose quorum. Our Custom Resource is configured with a [finalizer](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers) so that our Operator reconciles on CR deletion. In the deletion loop, we set a label on the StatefulSet. The label updates a file mounted in the RabbitMQ container via the [Kubernetes DownwardAPI](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/). In turn, this file is checked inside the PreStop hook to exit before the `rabbitmq-queues` CLI checks. We then remove the finalizer from the Custom Resource so it can be garbage collected and we avoid blocking.

```
if [ ! -z \"$(cat /etc/pod-info/skipPreStopChecks)\" ]
  then exit 0
fi
```

If a PreStop hook does not return, either because an API call hangs or the loop never resolves, then the Kubelet kills the container after the `terminationGracePeriodSeconds` and continues deleting the Pod. We have set the RabbitMQ container termination grace period to a week. This is an arbitrarily large number to ensure that a cluster Pod is not deleted if the checks prevent it and stays alive long enough for human intervention.

Key Considerations
===

### Downtime

While our aim is to provide zero-downtime upgrades, in practice several factors mean that at best we can seek to minimize any downtime experienced:
- This is a best effort approach. There is no transactional mechanism to ensure that state remains the same from when the lifecycle check gets run to when the Pod is allowed to terminate.
- We cannot account for misconfigured apps. If a RabbitMQ client is not configured to re-establish its connection on failure then manual intervention will be required. The client will also need to be connecting to the cluster via DNS or a load balancer (i.e. not through a hardcoded IP) in order to resolve to a running node when reconnecting.
- Even for properly configured apps, there will be downtime between losing the connection to the deleted node and connecting to another one.

### Balancing the cluster

RabbitMQ classic queues promote the oldest replica when a leader is lost. For StatefulSet deployed Pods this means that the leader is always the Pod that is about to be deleted (and then finishes as the first Pod to be rolled). We are actively considering approaches to reduce the number of leader elections during an upgrade and make sure that the cluster's queues as balanced between replicas once its over. One consequence of the current approach is that apps connected to the cluster will potentially have their connections reset multiple times if they are load balanced to a node which is in line for deletion.

Queue leaders can be manually rebalanced with the `rabbitmq-queues rebalance` [command](https://www.rabbitmq.com/rabbitmq-queues.8.html#Cluster)

### Implications for Kubernetes Cluster administration

 The PreStop hook will be run before all Pod deletions since we cannot distinguish/discriminate between a Pod 'upgrade' and deletion. StatefulSet rolling upgrades use the same Kubernetes mechanisms as regular pod deletion (and creation) to apply changes. Any cluster wide maintenance (e.g. restarting or replacing a kubernetes node that has a RabbitMQ Pod) must take this into consideration.

If required, pods can be deleted immediately by running `kubectl delete pod pod-name --force --grace-period=0`.

### Graceful shutdown of rabbitmq-server

The RabbitMQ server process on each Pod is [configured](https://github.com/rabbitmq/rabbitmq-server/pull/2227/files) to stop gracefully on receiving a SIGTERM or SIGQUIT from v3.7.x.
