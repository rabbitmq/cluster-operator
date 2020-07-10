# Additional PersistentVolumeClaim Example

You can leverage StatefulSet override (`spec.Override.StatefulSet`) to modify any property of the statefulSet deployed as part of a RabbitmqCluster instance. `spec.Override.StatefulSet` is in the format of appsv1.StatefulSetSpec. When provided, it will be applied as a patch to the RabbitmqCluster instance StatefulSet definition, using kubernetes [Strategic Merge Patch](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/strategic-merge-patch.md).

The example creates two PersistentVolumeClaim and mounts the additional PersistentVolumeClaim to the `rabbitmq` container.


```shell
kubectl apply -f rabbitmq.yaml
```
