# Pod Anti Affinity Example

This simple pod anti affinity rule will try to assign pods to different nodes according to [Assigning Pods to Nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/). 

The `preferredDuringSchedulingIgnoredDuringExecution` type can be replaced by `requiredDuringSchedulingIgnoredDuringExecution` to require pod separation as in production-ready example.

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```

And once deployed, you can check what defaults were applied like this (`spec` section is the most important):

```shell
kubectl get -o yaml rabbitmqclusters.rabbitmq.com hello-world
```