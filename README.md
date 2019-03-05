# rabbitmq-for-kubernetes
RabbitMQ for Kubernetes

## Requirements
You should have the following tools installed:
* kubectl
* [kustomize](https://github.com/kubernetes-sigs/kustomize/) (in the future it should become a part of `kubectl`)
* gcloud
* [kubebuilder](https://book.kubebuilder.io/quick_start.html)

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

## How to deploy with kustomize
1. Go to the `templates` folder
2. Set the current namespace to the namespace you are deploying to, for example:
```
kubectl config set-context $(kubectl config current-context) --namespace=rabbitmq
```
3. Edit `kustomization.yaml` - set `namePrefix` and `commonLabels` and/or `namespace` `erlang-cookie`
4. Generate and set the `erlang-cookie`
5. Run `kustomize build` to generate the manifest. You can send it directly to `kubectl` like this:
```
kustomize build | kubectl apply -f -
```

If this fails with a `forbidden: attempt to grant extra privileges` error, you need to grant yourself the Cluster Admin role:
```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user USERNAME@pivotal.io
```
More information about [Role Based Access](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control).

# RabbitMQ Kubernetes Manager

## How to set up the Manager for development

We are gitignoring the vendor directory because it is huge. When getting started, run `dep ensure` to pull dependencies. Add the `-v` flag to see progress or be patient and trust that the process isn't hanging.

## How to push the Manager docker image

1. `export IMG=eu.gcr.io/cf-rabbitmq/rabbitmq-k8s-manager`
2. `gcloud auth configure-docker`
3. `make docker-build`
4. `make docker-push`
5. To deploy, run `make deploy`

# Tear down

1. To delete the cluster run `kubectl delete -f {path to yaml used to deploy e.g. '/config/default/samples/rabbitmq_v1beta1_rabbitmqcluster.yaml}`
1. To delete the manager run (from the  '/config/default') `kustomize build | kubectl delete -f -`
1. If you've deployed a cluster directly using kustomize from the templates folder, delete the cluster by running (from 'templates') `kustomize build | kubectl delete -f -`
