# RabbitMQ for Kubernetes

## Compatibility and Upgrades
Features delivered in alpha are not guaranteed to be present in GA. As of now, there are no plans for upgrade or migration path from alpha.

## Pre-requisite
1. [docker](https://docs.docker.com/install/)
2. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

## Download Artifacts
The artifacts for RabbitMQ for Kubernetes can be downloaded from [Pivotal Network](https://network.pivotal.io/products/p-rabbitmq-for-kubernetes/). The artifact contains
three docker images and deployment manifests for the operator and the broker. The three images are:

1. RabbitMQ
2. RabbitMQ Operator
3. Service Broker

## Images
We have all of our images hosted in our gcr registry, and publicly available. You can either use the saved images from our artifact and push them to your own registry, or you can use our public images. Our public images are hosted in `us.gcr.io/cf-rabbitmq-for-k8s-bunny`. Skip the relocate images step if you want to use our public images.

## Relocate Images (optional)
// TODO: update image names after creating pivnet artifact
This step is only required if you want to use your own image registry. If you don't use a local image registry, skip this step.

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


## Configuring Image Repository

// TODO: update image names/urls after creating pivnet artifact and pushing public images

If you want to use our public images, provide our images url in our operator manifest (installation/operator.yaml)
Replace all references of "REPLACE-WITH-IMAGE-REPOSITORY-HOST" with your image repository host, `us.gcr.io/cf-rabbitmq-for-k8s-bunny`
Replace all references of "REPLACE-WITH-OPERATOR-IMAGE-URL" with the full operator image URL,  `us.gcr.io/cf-rabbitmq-for-k8s-bunny/rabbitmq-for-kubernetes-controller:0.1.0`

Provide our images url in the service broker manifest (installation/service-broker.yaml)
Replace all references of "REPLACE-WITH-BROKER-IMAGE-URL" with the full broker image URL, `us.gcr.io/cf-rabbitmq-for-k8s-bunny/rabbitmq-for-kubernetes-servicebroker:0.1.0`

If you want to use your own image registry, provide your repository url in our operator manifest (installation/operator.yaml)
Replace all references of "REPLACE-WITH-IMAGE-REPOSITORY-HOST" with your image repository host `<your-repository>`
Replace all references of "REPLACE-WITH-OPERATOR-IMAGE-URL" with the full operator image URL `<your-repository>/rabbitmq-for-kubernetes-operator:0.1.0`

Provide your repository url in the service broker manifest (installation/service-broker.yaml)
Replace all references of "REPLACE-WITH-BROKER-IMAGE-URL" with the full broker image URL `<your-repository>/rabbitmq-for-kubernetes-broker:0.1.0`

## Configuring Service Type (Optional)

Our operator allows you to specify what kind of Service is provisioned for your RabbitMQ cluster. The default type is ClusterIP.
You can find the detailed explanation of different Kubernetes Service types [here](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types)
If you wish to change the Service type, you can change it in our operator manifest (installation/operator.yaml):

Replace the value of "SERVICE_TYPE" from "ClusterIP" to either NodePort or LoadBalancer. (ExternalName is not supported)

If you have any trouble creating or connecting to your RabbitMQ cluster instance, please reach out to our team on Pivotal slack : #rabbitmq-for-k8s

## Deploy Operator and Broker

To deploy the operator and broker, and to install RabbitmqCluster Custom Resource Definition:
```
kubectl apply -f installation/
```

## Register Broker with Your CF
// TODO: add guidance on how to fetch the broker username and password
// This will be implemented in #167564232

In order to register the service broker, run the following CF command:

```
cf create-service-broker <service-broker-name> <broker-username> <broker-password> http://<service-broker-ip>:8080
```

The `<service-broker-name>` can be any arbitrary name. The `<service-broker-ip>` is the external IP assigned to the `LoadBalancer` service named `p-rmq-servicebroker`, which is deployed in the service broker namespace `pivotal-rabbitmq-servicebroker-system`.

Our service broker username and password is hardcoded. Username is `p1-rabbit`, and password is `p1-rabbit-testpwd`.

Once the service broker is registered, run the following command to enable access in the marketplace:

```
cf enable-service-access p-rabbitmq-k8s -b <service-broker-name>
```

## Limitations

### Updating the RabbitMQ Cluster
For now, there is no capability to update the RabbitMQ cluster and any of its child objects (stateful set, config map, and secrets) after creation i.e. if you update any of the configurations, they will not take any effect. In case you deleted any of the child objects (stateful set, config map, and secrets), they will not be recreated. You will have to delete the cluster and recreate it again.

### RabbitMQ Image
At the moment, we do not support pulling the RabbitMQ image from a repository that requires authentication.

### Service Broker
Right now, the service broker can only provision instances in the the Kubernetes cluster that the operator and broker are deployed. The broker creates a new namespace for each instance to live in.

### Provision Status of Service Instance
If the service instance provision status is "stuck" in "create in progress", it's possible that the RabbitMQ cluster has failed to create. We recommend you to check the status of the RabbitMQ cluster in your Kubernetes cluster for more details on the failure.
