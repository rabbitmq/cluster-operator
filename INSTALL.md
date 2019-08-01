# RabbitMQ for Kubernetes

1. Where to get the image from and where to put them
2. How to install the operator (including image name editing)
2. Explain how to grant Kubernetes worker nodes access to pull private images from a repository
3. How to deploy the service broker (from the prototype)
4. How to register the broker with PAS
5. Explain networking as much as possible and ask to reach out to us for help
6. Any assumptions and restrictions for this alpha (e.g. missing functionality, unsupported upgrades to later versions, rabbitmq management image cannot be private ...)

## Pre-requisite
1. [docker](https://docs.docker.com/v17.12/docker-cloud/installing-cli/)
2. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

## Download artefacts
The artefacts for RabbitMQ for Kubernetes can be downloaded from [Pivotal Network](https://network.pivotal.io/products/p-rabbitmq-for-kubernetes/). The artifact contains
three docker images and deployment manifests for the operator and the broker. The three images are:

1. RabbitMQ Operator
2. Service Broker
3. RabbitMQ binaries

## Relocate images
// TODO: update image names after creating pivnet artifact

Uncompress the image to local docker

```
tar xvf path/to/rabbitmq-for-kubernetes-artifact.tar
docker load -i operator-image
docker load -i rabbitmq-image
docker load -i servicebroker-image
```

Tag the image to point to your own image repository

```
docker tag rabbitmq-image <your-repository>/rabbitmq:3.8-rc-management
docker tag operator-image <your-repository>/rabbitmq-for-kubernetes-operator:<version>
docker tag servicebroker-image <your-repository>/rabbitmq-for-kubernetes-broker:<version>
```

Upload the image to your own image repository

```
docker push <your-repository>/rabbitmq:3.8-rc-management
docker push <your-repository>/rabbitmq-for-kubernetes-operator:<version>
docker push <your-repository>/rabbitmq-for-kubernetes-broker:<version>
```

## Configure Kubernetes cluster access to private images (optional)
We highly encourage you to keep the operator and service-broker images private if your repository is publicly accessible.

```
kubectl apply -f installation/namespace.yaml
```

Create a secret that authorises access to private images, in the `pivotal-rabbitmq-system` namespace that we just created. Instructions and more details [here](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)


## Deploy operator

Provide your repository url in our operator manifest
Replace all references of "REPLACE-WITH-IMAGE-REPOSITORY-HOST" with your image repository host, e.g. `eu.gcr.io`
Replace all references of "REPLACE-WITH-OPERATOR-IMAGE-URL" with the full operator image URL, e.g. `eu.gcr.io/my-project-name/rabbitmq-for-kubernetes-operator:0.1.0-alpha.1`
Replace all references of "REPLACE-WITH-BROKER-IMAGE-URL" with the full broker image URL, e.g. `eu.gcr.io/my-project-name/rabbitmq-for-kubernetes-broker:0.1.0-alpha.1`

Install RabbitmqCluster Custom Resource Definition and Operator
```
kubectl apply -f installation/
```

## Deploy broker

## Register broker with your CF