## Deploy service broker

Currently we deploy the service broker by running:

```
kubectl run --namespace rabbitmq-for-kubernetes-servicebroker --generator=run-pod/v1 --image eu.gcr.io/cf-rabbitmq/rabbitmq-k8s-servicebroker servicebroker --port 8080 --expose
kubectl apply -f servicebroker-lb.yaml
```
