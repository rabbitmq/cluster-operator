# rabbitmq-for-kubernetes
RabbitMQ for Kubernetes

## Requirements
You should have the following tools installed:
* kubectl
* gcloud
* [kubebuilder](https://book.kubebuilder.io/quick_start.html)
* [counterfeiter](https://github.com/maxbrunsfeld/counterfeiter) (run `go get -u github.com/maxbrunsfeld/counterfeiter`)

## How to get started
### GKE
Most of the time we deploy to our GKE cluster. To start working against it, do the following:
```
gcloud container clusters get-credentials cluster-1 --zone europe-west1-c --project cf-rabbitmq
```
Core team members and other non-tile team collaborators, need to be added to the `cf-rabbitmq` Google Cloud project first.

### Kind
Kind is a local environment deployer similar to minikube but multi-node. You can deploy to it by:
1. [Installing kind](https://github.com/kubernetes-sigs/kind#installation-and-usage)
2. Running
```
kind create cluster
```
3. Switching `kubectl` to target local cluster using
```
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
```

## How to deploy a RabbitMQ cluster manually
1. Edit `templates/kustomization.yaml` - set `namePrefix` and `commonLabels` and/or `namespace` `erlang-cookie`
2. Generate and set the `erlang-cookie`
3. Run
```
kubectl apply -k templates
```

If this fails with a `forbidden: attempt to grant extra privileges` error, you need to grant yourself the Cluster Admin role:
```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user USERNAME@pivotal.io
```
More information about [Role Based Access](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control).

# RabbitMQ Kubernetes Operator

## How to set up the Operator for development

We are gitignoring the vendor directory because it is huge. When getting started, run `dep ensure` to pull dependencies. Add the `-v` flag to see progress or be patient and trust that the process isn't hanging.

## How to push the Operator docker image

1. gcloud config set project cf-rabbitmq
2. `gcloud auth configure-docker`
3. `make image`
4. To deploy, run `make deploy`

# Tear down

1. To delete the cluster run `kubectl delete -f {path to yaml used to deploy e.g. '/config/default/samples/rabbitmq_v1beta1_rabbitmqcluster.yaml}`
1. To delete the operator run `kubectl delete -k config/default`
1. If you've deployed a cluster manually, delete the cluster by running `kubectl delete -k templates`

## Deploy a new operator and service broker (e.g. for acceptance)

1. `make delete`
2. `make deploy_all`
3. `make gcr_viewer_service_account` - if this fails with an error about exhausted key quota delete some of the keys in the service account in GCP
3. If only testing the operator: `make single`, `make ha`
4. If testing the broker, `make register_servicebroker` and continue in PCF
