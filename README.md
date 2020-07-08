# RabbitMQ for Kubernetes Operator

The RabbitMQ for Kubernetes Operator is a way of managing [RabbitMQ](https://www.rabbitmq.com/) clusters deployed to [Kubernetes](https://kubernetes.io/). RabbitMQ for Kubernetes has been built using the [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) implementation of the [operator pattern](https://coreos.com/blog/introducing-operators.html). This repository contains a [custom controller](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers) and [custom resource definition (CRD)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) designed for the lifecyle (creation, upgrade, graceful shutdown) of a RabbitMQ cluster.

**Note**: this repository is under active development and is provided as **beta** software. Official support for this software is not provided; if you encounter any issues running this software, please feel free to [contribute to the project](#contributing).

## Supported Versions

The operator deploys RabbitMQ `3.8.5`, and requires a Kubernetes cluster of `1.16` or above.

## Quickstart

### Deploying on KinD

The easiest way to set up a local development environment for running the RabbitMQ operator is using [KinD](https://kind.sigs.k8s.io/):

1. Follow the KinD [installation guide](https://kind.sigs.k8s.io/#installation-and-usage) to deploy a Kubernetes cluster
1. Run `make deploy-kind`
1. Check that the operator is running by running `kubectl get all --namespace=rabbitmq-system`
1. Deploy a `RabbitMQCluster` custom resource. Refer to the [example YAML](./cr-example.yaml) and [documentation](https://docs.pivotal.io/rabbitmq-kubernetes/0-7/using.html#configure) for available CR attributes
    1. Due to resource limitations on your Docker daemon, the Kubernetes might not be able to schedule all `RabbitmqCluter` nodes. Either [increase your Docker daemon's resource limits](https://docs.docker.com/docker-for-mac/#resources) or deploy the `RabbitmqCluster` custom resource with `resources: {}` to remove default `memory` and `cpu` resource settings.
    1. If you set the `serviceType` to `LoadBalancer`, run `make prepare-kind` to deploy a [MetalLB](https://metallb.universe.tf/) load balancer. This will allow the operator to complete the `RabbitmqCluster` provisioning by assign an arbitrary local IP address to the cluster's client service. Proper [network configuration](https://metallb.universe.tf/installation/network-addons/) is required to route traffic via the assigned IP address.

### Documentation

The RabbitMQ for Kubernetes [documentation](https://docs.pivotal.io/rabbitmq-kubernetes/0-7/index.html) has steps to deploy to a Kubernetes cluster:
- [Deploying an operator](https://docs.pivotal.io/rabbitmq-kubernetes/0-7/installing.html)
- [Deploying a RabbitMQ cluster](https://docs.pivotal.io/rabbitmq-kubernetes/0-7/using.html)
- [Monitoring the cluster](https://docs.pivotal.io/rabbitmq-kubernetes/0-7/monitoring.html)


### Makefile

#### Required environment variables

- DOCKER_REGISTRY_SERVER: URL of docker registry containing the Operator image (e.g. `registry.my-company.com`)
- DOCKER_REGISTRY_USERNAME: Username for accessing the docker registry
- DOCKER_REGISTRY_PASSWORD: Password for accessing the docker registry
- DOCKER_REGISTRY_SECRET: Name of Kubernetes secret in which to store the Docker registry username and password
- OPERATOR_IMAGE: path to the Operator image within the registry specified in DOCKER_REGISTRY_SERVER (e.g. `rabbitmq/rabbitm-operator`). Note: OPERATOR_IMAGE should **not** include a leading slash (`/`)

#### Make targets

- **controller-gen** Download controller-gen if not in $PATH
- **deploy** Deploy operator in the configured Kubernetes cluster in ~/.kube/config
- **deploy-dev** Deploy operator in the configured Kubernetes cluster in ~/.kube/config, with local changes
- **deploy-kind** Load operator image and deploy operator into current KinD cluster
- **deploy-sample** Deploy RabbitmqCluster defined in config/sample/base
- **destroy** Cleanup all operator artefacts
- **kind-prepare** Prepare KinD to support LoadBalancer services, and local-path StorageClass
- **kind-unprepare** Remove KinD support for LoadBalancer services, and local-path StorageClass
- **list** List Makefile targets
- **run** Run operator binary locally against the configured Kubernetes cluster in ~/.kube/config
- **unit-tests** Run unit tests
- **integration-tests** Run integration tests
- **system-tests** Run end-to-end tests against Kubernetes cluster defined in ~/.kube/config

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please either comment on an [existing issue](https://github.com/pivotal/rabbitmq-for-kubernetes/issues), or file a new one.

### Testing

Before submitting a pull request, ensure all local tests pass:
- `make unit-tests`
- `make integration-tests`

<!-- TODO: generalise deployment process: make DOCKER_REGISTRY_SECRET and DOCKER_REGISTRY_SERVER configurable -->
Also, run the system tests against a Kubernetes cluster:
- `make deploy`
- `make system-tests`

### Code Conventions

This project follows the same code conventions as the [kubernetes golang code conventions](https://github.com/kubernetes/community/blob/master/contributors/guide/coding-conventions.md#code-conventions). The kuberentes golang code conventions mostly refer to [Effective Go](https://golang.org/doc/effective_go.html) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments). Please ensure your pull requests follow these guidlines.

## License

[Licensed under the MPL](LICENSE.txt), same as RabbitMQ server.

## Copyright

Copyright 2020 VMware, Inc. All Rights Reserved.

