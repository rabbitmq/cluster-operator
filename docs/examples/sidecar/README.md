# Sidecar Example

This example adds an additional container in the rabbitmq pod using the StatefulSet override. You can add multiple additional containers and add additional containers to `initContainers` as well.

You can deploy this example:

```shell
kubectl apply -f rabbitmq.yaml
```
