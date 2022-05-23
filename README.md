# RabbitMQ Cluster Kubernetes Operator

Kubernetes operator to deploy and manage [RabbitMQ](https://www.rabbitmq.com/) clusters. This repository contains a [custom controller](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers) and [custom resource definition (CRD)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) designed for the lifecycle (creation, upgrade, graceful shutdown) of a RabbitMQ cluster.

## Quickstart

If you have a running Kubernetes cluster and `kubectl` configured to access it, run the following command to install the operator:

```bash
kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml
```

Then you can deploy a RabbitMQ cluster:

```bash
kubectl apply -f https://raw.githubusercontent.com/rabbitmq/cluster-operator/main/docs/examples/hello-world/rabbitmq.yaml
```

<p align="center">
  <img width="100%" src="./docs/demos/installation.svg">
</p>

## Documentation

RabbitMQ Cluster Kubernetes Operator is covered by several guides:

 - [Operator overview](https://www.rabbitmq.com/kubernetes/operator/operator-overview.html)
 - [Deploying an operator](https://www.rabbitmq.com/kubernetes/operator/install-operator.html)
 - [Deploying a RabbitMQ cluster](https://www.rabbitmq.com/kubernetes/operator/using-operator.html)
 - [Monitoring the cluster](https://www.rabbitmq.com/kubernetes/operator/operator-monitoring.html)
 - [Troubleshooting operator deployments](https://www.rabbitmq.com/kubernetes/operator/troubleshooting-operator.html)

In addition, a number of [examples](./docs/examples) can be found in this repository.

The doc guides are open source. The source can be found in the [RabbitMQ website repository](https://github.com/rabbitmq/rabbitmq-website/)
under `site/kubernetes`.

## Supported Versions

The operator deploys RabbitMQ `3.10.2` by default, and supports versions from `3.8.8` upwards. The operator requires Kubernetes `1.19` or newer.

## Versioning

RabbitMQ Cluster Kubernetes Operator follows non-strict [semver](https://semver.org/).

[The versioning guidelines document](version_guidelines.md) contains guidelines
on how we implement non-strict semver. The version number MAY or MAY NOT follow the semver rules. Hence, we highly recommend to read
the release notes to understand the changes and their potential impact for any release.

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please either comment on an [existing issue](https://github.com/rabbitmq/cluster-operator/issues), or file a new one.

Please read [contribution guidelines](CONTRIBUTING.md) if you are interested in contributing to this project.

## License

[Licensed under the MPL](LICENSE.txt), same as RabbitMQ server.

## Copyright

Copyright 2020-2021 VMware, Inc. All Rights Reserved.
