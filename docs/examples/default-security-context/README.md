# Setting Pod Security Context to Container Runtime default

By default, the RabbitMQ Cluster Operator applies a [securityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/) in order to run the RabbitMQ container
and initContainer as a specific non-root user.

In some deployments, you may wish to remove this securityContext so that the containers are run with the default securityContext of the container runtime. For example, in Openshift, in order
to [run the RabbitMQ containers as an arbitrary user](https://www.openshift.com/blog/a-guide-to-openshift-and-uids), you will need to remove the operator-configured securityContext.

Note that unless your Kubernetes distribution applies a default securityContext to pods, your containers will run as root.

## Example

The example `rabbitmq.yaml` contains an override which will set the securityContext to the default, by specifying it as an empty struct (`{}`).

```shell
kubectl apply -f rabbitmq.yaml
```

You can then inspect the container to check that it is running with the default securityContext:
```shell
kubectl exec default-security-context-server-0 -- id
uid=0(root) gid=0(root) groups=0(root)
```

Or, in an environment where the runtime provides a default securityContext, like Openshift:
```shell
kubectl exec default-security-context-server-0 -- id
uid=1000620000(1000620000) gid=0(root) groups=0(root),1000620000
```
