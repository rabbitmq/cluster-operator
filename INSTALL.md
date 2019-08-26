# RabbitMQ for Kubernetes

## Compatibility and Upgrades
Features delivered in alpha are not guaranteed to be present in GA. As of now, there are no plans for an upgrade or migration path from alpha.

## Pre-requisite
1. [Docker](https://docs.docker.com/install/)
2. Working Kubernetes cluster
3. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

This installation guide is written with the assumption that you are using a private image registry. If you don't have access to an image registry yourself, please contact the team on Pivotal slack (#rabbitmq-for-k8s)

## Download Artifacts
The artefact for RabbitMQ for Kubernetes can be downloaded from [Pivotal Network](https://network.pivotal.io/products/p-rabbitmq-for-kubernetes/). The artefact contains
three docker images and deployment manifests for the operator and the broker. The three images are:

1. RabbitMQ
2. RabbitMQ Operator
3. Service Broker

### Relocate Images

Load the images to local Docker:

```
tar xvf path/to/rabbitmq-for-kubernetes-<version>.tar
docker load -i rabbitmq-for-kubernetes-operator
docker load -i rabbitmq-3.8-rc-management
docker load -i rabbitmq-for-kubernetes-servicebroker
```

Tag the image to point to your own image repository:

<your-repository> would be <image-repository-host>:<image-repository-port>/<image-subpath>
without the schema (e.g. http:// or https://)

```bash
~$ docker tag rabbitmq-3.8-rc-management \
>  <your-repository>/rabbitmq:3.8-rc-management
~$ docker tag rabbitmq-for-kubernetes-operator \
>  <your-repository>/rabbitmq-for-kubernetes-operator:<version>
~$ docker tag rabbitmq-for-kubernetes-servicebroker \
>  <your-repository>/rabbitmq-for-kubernetes-servicebroker:<version>
```

Push the image to your own image repository:

```
docker push <your-repository>/rabbitmq:3.8-rc-management
docker push <your-repository>/rabbitmq-for-kubernetes-operator:<version>
docker push <your-repository>/rabbitmq-for-kubernetes-servicebroker:<version>
```

### Configure Kubernetes cluster access to private images (optional)
We highly encourage you to keep the operator and service-broker images private if your repository is publicly accessible.

```
kubectl apply -f manifests/namespace.yaml
```

In your cluster, create a Kubernetes secret that authorises access to private images, in the `pivotal-rabbitmq-system` namespace that we just created. Repeat this task for the `pivotal-rabbitmq-servicebroker-system` namespace. Instructions and more details [here](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)


### Configure Image Repository

Provide your repository url in our operator manifest (`manifests/operator.yaml`)
Replace all references of "REPLACE-WITH-IMAGE-REPOSITORY-HOST" with your image repository host `<your-repository>`
Replace all references of "REPLACE-WITH-OPERATOR-IMAGE-URL" with the full operator image URL:

`<your-repository>/rabbitmq-for-kubernetes-operator:<version>`

Provide your repository url in the service broker manifest (`manifests/service-broker.yaml`)
Replace all references of "REPLACE-WITH-BROKER-IMAGE-URL" with the full broker image URL:

`<your-repository>/rabbitmq-for-kubernetes-servicebroker:<version> `

## Configuring Service Type (Optional)

Our operator allows you to specify what kind of Kubernetes Service is provisioned for your RabbitMQ cluster. The default type is ClusterIP.
You can find the detailed explanation of different Kubernetes Service types [here](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types)
If you wish to change the Service type, you can change it in our operator manifest (`manifests/operator.yaml`):

Replace the value of `SERVICE:TYPE` from `ClusterIP` to either `NodePort` or `LoadBalancer`. Please note, ExternalName is currently not supported.

### Configure Service Annotations (Optional)

It is possible to configure custom annotations to the Service deployed for the RabbitMQ cluster. By default there are not annotations configured.

If you wish to configure annotations, you can change it in our operator manifest (`manifests/operator.yaml`):

Add list of key-value pairs to `ANNOTATIONS` under the `SERVICE` object, e.g.

```yaml
data:
  SERVICE: |
    TYPE: NodePort
    ANNOTATIONS:
      service.beta.kubernetes.io/aws-load-balancer-internal: 0.0.0.0/0
```


## Create Broker Credentials

The service broker looks for its username and password in a Kubernetes secret called `broker-credentials` in the `pivotal-rabbitmq-system` namespace. To create a secret follow the steps below:

```bash
echo -n <broker-username> > ./username
echo -n <broker-password> > ./password
kubectl create secret generic broker-credentials -n pivotal-rabbitmq-system --from-file=./username --from-file=./password
```

There are several ways to create a secret, you can refer to the [official documentation](https://kubernetes.io/docs/concepts/configuration/secret) for more details.
Keep in mind that the secret name should be `broker-credetails` and should be in the `pivotal-rabbitmq-system` namespace with username and password as part of data, otherwise the service broker will fail to deploy.

## Deploy Operator and Broker

To deploy the operator and broker, and to install the `RabbitmqCluster` Custom Resource Definition:
```
kubectl apply -f manifests/
```

## Register Broker with Cloud Foundry

In order to register the service broker, run the following `cf` CLI command:

```
~$ cf create-service-broker <service-broker-name> <broker-username> <broker-password> \
>  http://<service-broker-ip>:8080
```

The `<service-broker-name>` can be any arbitrary name. The `<service-broker-ip>` is the external IP assigned to the `LoadBalancer` service named `p-rmq-servicebroker`, which is deployed in the service broker namespace `pivotal-rabbitmq-servicebroker-system`.

The broker-username and broker-password is the credentials you've used to create the kubernetes secrets in previous step "Create Broker Credentials".

Once the service broker is registered, run the following command to enable access in the marketplace:

```
cf enable-service-access p-rabbitmq-k8s -b <service-broker-name>
```

## Limitations

### Updating the RabbitMQ Cluster
For now, there is no capability to update the RabbitMQ cluster and any of its child objects (stateful set, config map, service and secrets) after creation i.e. if you update any of the configurations, they will not take any effect. In case you deleted the child config map, service and/or secret objects, they will not be recreated (stateful set objects will be recreated). You will have to delete the cluster and recreate it again.

### RabbitMQ Image
At the moment, we do not support pulling the RabbitMQ image from a repository that requires authentication.

### Service Broker
Right now, the service broker can only provision instances in the the Kubernetes cluster that the operator and broker are deployed. The broker creates a new namespace for each instance to live in.

### Provision Status of Service Instance
If the service instance provision status is stuck in "create in progress", it's possible that the RabbitMQ cluster has failed to create. We recommend you to check the status of the RabbitMQ cluster resources in your Kubernetes cluster for more details on the failure.
