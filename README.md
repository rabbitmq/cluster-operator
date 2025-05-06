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

 - [Operator overview](https://www.rabbitmq.com/kubernetes/operator/operator-overview)
 - [Deploying an operator](https://www.rabbitmq.com/kubernetes/operator/install-operator)
 - [Deploying a RabbitMQ cluster](https://www.rabbitmq.com/kubernetes/operator/using-operator)
 - [Monitoring the cluster](https://www.rabbitmq.com/kubernetes/operator/operator-monitoring)
 - [Troubleshooting operator deployments](https://www.rabbitmq.com/kubernetes/operator/troubleshooting-operator)

In addition, a number of [code examples](./docs/examples) can be found in this repository.

The doc guides are open source. The source can be found in the [RabbitMQ website repository](https://github.com/rabbitmq/rabbitmq-website/)
under `site/kubernetes`.

## Supported Versions

The operator deploys RabbitMQ `4.1.0` by default, and should work with [any supported RabbitMQ version](https://www.rabbitmq.com/release-information) and [Kubernetes version](https://kubernetes.io/releases/).

## Versioning

RabbitMQ Cluster Kubernetes Operator follows non-strict [semver](https://semver.org/).

[The versioning guidelines document](version_guidelines.md) contains guidelines
on how we implement non-strict semver. The version number MAY or MAY NOT follow the semver rules. Hence, we highly recommend to read
the release notes to understand the changes and their potential impact for any release.

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please either comment on an [existing issue](https://github.com/rabbitmq/cluster-operator/issues), or file a new one.

Please read [contribution guidelines](CONTRIBUTING.md) if you are interested in contributing to this project.

## Releasing

To release a new version of the Cluster Operator, create a versioned tag (e.g. `v1.2.3`) of the repo, and the release pipeline will
generate a new draft release, alongside release artefacts.

## License

[Licensed under the MPL](LICENSE.txt), same as RabbitMQ server.

## Copyright

(c) 2007-2024 Broadcom. All Rights Reserved. The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.

[![Go Report Card](https://goreportcard.com/badge/github.com/rabbitmq/cluster-operator)](https://goreportcard.com/report/github.com/rabbitmq/cluster-operator)
