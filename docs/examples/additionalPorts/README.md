# Additional Port Example

You can leverage the Client Service override (`spec.override.clientService`) and the StatefulSet override (`spec.override.statefulSet`) to modify any property of the Client Service and StatefulSet deployed as part of a RabbitmqCluster instance.

`spec.override.statefulSet` is in the format of appsv1.StatefulSet; and `spec.override.clientService` is in the format of corev1.Service. When provided, both overrides will be applied as patches using kubernetes [Strategic Merge Patch](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/strategic-merge-patch.md).

The example rabbitmqcluster manifests defines an additional port on the Client Service and the `rabbitmq` container itself.


```shell
kubectl apply -f rabbitmq.yaml
```
