# Additional Port Example

You can leverage the Service override (`spec.override.service`) and the StatefulSet override (`spec.override.statefulSet`) to modify any property of the Service and StatefulSet deployed as part of a RabbitmqCluster instance.

`spec.override.statefulSet` is in the format of appsv1.StatefulSet; and `spec.override.service` is in the format of corev1.Service. When provided, both overrides will be applied as patches using kubernetes [Strategic Merge Patch](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/strategic-merge-patch.md).

The example rabbitmqcluster manifests defines an additional port on the Service and the `rabbitmq` container itself.


```shell
kubectl apply -f rabbitmq.yaml
```
