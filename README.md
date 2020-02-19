# RabbitMQ for Kubernetes Operator

The RabbitMQ for Kubernetes Operator is a way of managing [RabbitMQ](https://www.rabbitmq.com/) clusters deployed to [Kubernetes](https://kubernetes.io/). RabbitMQ for Kubernetes has been built using the [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) implementation of the [operator pattern](https://coreos.com/blog/introducing-operators.html). This repository contains a [custom controller](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers) and [custom resource definition (CRD)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) designed for the lifecyle (creation, upgrade, graceful shutdown) of a RabbitMQ cluster.

## Quickstart

### Deploying on KinD

The easiest way to set up a local development environment for running the RabbitMQ operator is using [KinD](https://kind.sigs.k8s.io/):

1. Follow the KinD [installation guide](https://kind.sigs.k8s.io/#installation-and-usage) to deploy a Kubernetes cluster
1. Run `make deploy-kind`
1. Check that the operator is running by running `kubectl get all`
1. Deploy a `RabbitMQCluster` custom resource. Refer to the [example YAML](./cr-example) and [documentation](http://docs-pcf-staging.cfapps.io/rabbitmq-kubernetes/0-6/using.html) for available CR attributes
    1. Due to resource limitations on your Docker daemon, the Kubernetes might not be able to schedule all `RabbitmqCluter` nodes. Either [increase your Docker daemon's resource limits](https://docs.docker.com/docker-for-mac/#resources) or deploy the `RabbitmqCluster` custom resource with `resources: {}` to remove default `memory` and `cpu` resource settings.
    1. If you set the `serviceType` to `LoadBalancer`, run `make prepare-kind` to deploy a [MetalLB](https://metallb.universe.tf/) load balancer. This will allow the operator to complete the `RabbitmqCluster` provisioning by assign an arbitrary local IP address to the cluster's ingress service. Proper [network configuration](https://metallb.universe.tf/installation/network-addons/) is required to route traffic via the assigned IP address.

### Deploying with Minikube

// TODO

### Documentation

The RabbitMQ for Kubernetes [documentation](http://docs-pcf-staging.cfapps.io/rabbitmq-kubernetes/0-6/index.html) has steps for production deployments:
- [deploying an operator](http://docs-pcf-staging.cfapps.io/rabbitmq-kubernetes/0-6/installing.html)
- [deploying a RabbitMQ cluster](http://docs-pcf-staging.cfapps.io/rabbitmq-kubernetes/0-6/using.html)
- [monitoring the cluster](http://docs-pcf-staging.cfapps.io/rabbitmq-kubernetes/0-6/monitoring.html)

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please either comment on an [existing issue](https://github.com/pivotal/rabbitmq-for-kubernetes/issues), or file a new one.

### Testing

Before submitting a pull request, ensure all local tests pass:
- `make unit-tests`
- `make integration-tests`

## License

//TODO
