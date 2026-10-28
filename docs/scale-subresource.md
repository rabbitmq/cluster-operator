# RabbitmqCluster Scale Subresource

## Overview

The `RabbitmqCluster` CRD exposes the Kubernetes `/scale` subresource. This lets generic tooling such as `kubectl scale` and [KEDA](https://keda.sh/) change the number of replicas of a RabbitMQ cluster without knowing anything about the `RabbitmqCluster` API.

The main use case is **scale to zero**: putting idle clusters to sleep (for example per-branch test environments) and waking them up again when they are needed. Going from N replicas to 0 and back to N is supported. Autoscaling a running cluster between 1 and N replicas is deliberately **not** supported (see [Why the HorizontalPodAutoscaler Is Not Supported](#why-the-horizontalpodautoscaler-is-not-supported)).

## How It Works

The scale subresource maps onto existing fields of the `RabbitmqCluster`:

| Scale field | RabbitmqCluster field | Description |
|-------------|-----------------------|-------------|
| `spec.replicas` | `.spec.replicas` | Desired number of RabbitMQ nodes |
| `status.replicas` | `.status.replicas` | Number of pods currently reported by the StatefulSet |
| `status.selector` | *(not set)* | Intentionally left empty |

A write through `/scale` is just a write to `spec.replicas`. No special code path exists for it, so every rule the operator applies to `spec.replicas` also applies to `/scale`:

1. **Scaling up** (N to M, where M > N) is allowed.
2. **Scaling to zero** (N to 0) is allowed. Before the StatefulSet is scaled to zero, the operator saves the previous replica count in the `rabbitmq.com/before-zero-replicas-configured` annotation.
3. **Scaling from zero** (0 to N) is allowed only if N equals the count saved in the annotation. Once the cluster is restored, the annotation is removed.
4. **Scaling down** a running cluster (N to M, where 0 < M < N) is rejected.

The operator fills in `status.replicas` from the StatefulSet's `status.replicas` on every reconciliation. It is `0` (and omitted from the status) when the StatefulSet has no pods or does not exist yet.

### What Happens When a Scale Request Is Rejected

When a request breaks rule 3 or 4, the operator does not change the StatefulSet. Instead it:

- Records a `Warning` event with reason `UnsupportedOperation`
- Sets the `ReconcileSuccess` condition to `False` with an explanatory message
- Stops reconciling the cluster until `spec.replicas` is set back to a supported value

`spec.replicas` keeps the rejected value, so the cluster stays in this state until someone corrects it.

## Using the Scale Subresource

### Using kubectl

```bash
# Put the cluster to sleep
kubectl scale rabbitmqcluster my-rabbit --replicas=0

# Check the saved replica count
kubectl get rabbitmqcluster my-rabbit \
  -o jsonpath='{.metadata.annotations.rabbitmq\.com/before-zero-replicas-configured}'

# Wake it up again (must match the saved count)
kubectl scale rabbitmqcluster my-rabbit --replicas=3

# Read the scale subresource directly
kubectl get --raw /apis/rabbitmq.com/v1beta1/namespaces/default/rabbitmqclusters/my-rabbit/scale
```

### Using KEDA

KEDA scales between `idleReplicaCount` and `minReplicaCount` on its own, through `/scale`. Set `minReplicaCount` and `maxReplicaCount` to the cluster's replica count so that KEDA only ever switches between 0 and N:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: my-rabbit
spec:
  scaleTargetRef:
    apiVersion: rabbitmq.com/v1beta1
    kind: RabbitmqCluster
    name: my-rabbit
  idleReplicaCount: 0
  minReplicaCount: 1  # must equal spec.replicas of the cluster
  maxReplicaCount: 1  # must equal minReplicaCount
  triggers:
    # any trigger that signals the environment is in use
```

KEDA also creates a HorizontalPodAutoscaler for every `ScaledObject`. Because `/scale` exposes no selector, that HPA reports that the target is missing a selector and does not act. This is expected: it is what prevents the cluster from being autoscaled between 1 and N.

### Example Output

```yaml
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: my-rabbit
  namespace: default
  annotations:
    rabbitmq.com/before-zero-replicas-configured: "3"
spec:
  replicas: 0
status:
  conditions:
  - type: ReconcileSuccess
    status: "True"
```

After scaling back to 3 and once the pods are running:

```yaml
spec:
  replicas: 3
status:
  replicas: 3
```

## Why the HorizontalPodAutoscaler Is Not Supported

The scale subresource intentionally leaves out `selectorpath`, so `/scale` never reports a label selector. The HorizontalPodAutoscaler needs that selector to find the pods it scales, whatever the metric type. Without it, the HPA refuses to manage a `RabbitmqCluster`.

This is a deliberate design decision:

1. **Scaling down a running cluster is not supported.** Removing a RabbitMQ node can lose quorum queue replicas, classic queue data and stream data unless queues are drained and membership is changed first. The operator does not do this automatically, so it rejects scale-down requests.
2. **An autoscaler that can only scale up is a ratchet.** An HPA would add nodes during a traffic spike and then fail on every attempt to remove them, leaving the cluster stuck in `ReconcileSuccess=False`.
3. **Horizontal scaling is a capacity-planning decision.** Changing the number of nodes in a RabbitMQ cluster changes quorum sizes, queue placement and failure tolerance. A human operator should make that decision, not a metric threshold.

Scale to zero is different. Stopping every node and starting the same nodes again from their persistent volumes does not change cluster membership, so for RabbitMQ it is equivalent to a full cluster restart.

## Limitations

### 1. Scaling from Zero Must Restore the Previous Size

A cluster scaled to zero from 3 replicas can only be brought back with `--replicas=3`. Any other value is rejected until `spec.replicas` is set to 3. If you want a different size, restore the cluster first and then scale it up.

### 2. Data Survives Only If the Volumes Survive

Scale to zero relies on the StatefulSet keeping its PersistentVolumeClaims while it has no pods, which is the default. If the StatefulSet is configured with a `persistentVolumeClaimRetentionPolicy` that deletes claims on scale-down (for example through `spec.override`), the nodes start with empty volumes when the cluster is woken up.

### 3. Paused Clusters Are Not Reconciled

If the `rabbitmq.com/pauseReconciliation` label is set, `/scale` writes still update `spec.replicas`, but the operator neither applies them nor updates `status.replicas` until reconciliation resumes.

### 4. Wake-Up Is Not Instant

Scaling from zero starts every RabbitMQ node again. Clients will be refused connections until the nodes have booted and the cluster has formed, so configure KEDA triggers and client retries with this delay in mind.

## Troubleshooting

### Scaling Has No Effect

**Possible causes:**
1. The requested change is a scale-down of a running cluster
2. The cluster is being scaled from zero to a different size from the saved one
3. Reconciliation is paused

**Debug steps:**
```bash
# Check the ReconcileSuccess condition
kubectl get rabbitmqcluster my-rabbit \
  -o jsonpath='{.status.conditions[?(@.type=="ReconcileSuccess")]}'

# Check for UnsupportedOperation events
kubectl get events --field-selector involvedObject.name=my-rabbit,reason=UnsupportedOperation

# Check the saved replica count and the pause label
kubectl get rabbitmqcluster my-rabbit -o jsonpath='{.metadata.annotations}{"\n"}{.metadata.labels}'
```

**Fix:** set `spec.replicas` back to a supported value, for example the count in `rabbitmq.com/before-zero-replicas-configured`.

### KEDA HPA Reports a Missing Selector

This is expected; see [Using KEDA](#using-keda). KEDA can still switch the cluster between 0 and N. Make sure `minReplicaCount` and `maxReplicaCount` both equal the cluster's replica count.

### `status.replicas` Does Not Change

`status.replicas` mirrors the StatefulSet, which counts pods that exist, including pods that are still starting or terminating. Check the StatefulSet directly:

```bash
kubectl get statefulset my-rabbit-server -o jsonpath='{.status.replicas}'
kubectl get pods -l app.kubernetes.io/name=my-rabbit
```

## Best Practices

1. **Use scale to zero for idle, non-critical environments**: development, test and per-branch environments
2. **Keep KEDA between 0 and N**: always set `minReplicaCount` equal to `maxReplicaCount`
3. **Resize clusters manually**: change `spec.replicas` (or run `kubectl scale`) as a deliberate, reviewed operation
4. **Do not point a plain HPA at a RabbitmqCluster**: it is not supported, and it will not act
5. **Keep persistent volumes**: do not delete PVCs on scale-down if you expect the cluster to keep its data when it wakes up

## References

- [Kubernetes Scale Subresource](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#scale-subresource)
- [KEDA ScaledObject Specification](https://keda.sh/docs/latest/reference/scaledobject-spec/)
- [Horizontal Pod Autoscaling](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- [StatefulSet PersistentVolumeClaim Retention](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#persistentvolumeclaim-retention)
- [Issue #2275](https://github.com/rabbitmq/cluster-operator/issues/2275)
