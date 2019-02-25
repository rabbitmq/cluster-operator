# rabbitmq-for-kubernetes
RabbitMQ for Kubernetes

## Requirements
You should have the following tools installed:
* kubectl
* [kustomize](https://github.com/kubernetes-sigs/kustomize/) (in the future it should become a part of `kubectl`)
* gcloud

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

## How to deploy
1. Go to the `manifests` folder
2. Edit `kustomization.yaml` - set `namePrefix` and `commonLabels` and/or `namespace` `erlang-cookie`
3. Generate and set the `erlang-cookie`
4. Run `kustomize build` to generate the manifest. You can send it directly to `kubectl` like this:
```
kustomize build | kubectl apply -f -
```

If this fails with a `forbidden: attempt to grant extra privileges` error, you need to grant yourself the Cluster Admin role:
```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user USERNAME@pivotal.io
```
More information about [Role Based Access](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control).