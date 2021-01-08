# TLS Example

You can enable TLS by setting `.spec.tls.secretName` to the name of a secret containing TLS certificate and key.

First, you need to create the Secret which will contain the public certificate and private key to be used for TLS on the RabbitMQ nodes.
Assuming you already have these created and accessible as `server.pem` and `server-key.pem`, respectively, this Secret can be created by running:

```shell
kubectl create secret tls tls-secret --cert=server.pem --key=server-key.pem
```

Alternatively, this secret can also be created by [Cert Manager](https://cert-manager.io/).

Once the secret exists, you can deploy this example as follows:

```shell
kubectl apply -f rabbitmq.yaml
```

## SAN attributes for certificates

Make sure that the certificate's Subject Alternative Name (SAN) contains at least the following attributes:
* `*.<RabbitMQ cluster name>-nodes.<namespace>.svc.<K8s cluster domain name>`
* `<RabbitMQ cluster name>.<namespace>.svc.<K8s cluster domain name>`

If wildcards are not permitted, you must provide a SAN attribute for each RabbitMQ node in your RabbitMQ cluster.
For example, if you deploy a 3-node RabbitMQ cluster named `myrabbit` in namespace `mynamespace` with the default Kubernetes cluster domain `cluster.local`, the SAN must include at least the following attributes:
* `myrabbit-server-0.myrabbit-nodes.mynamespace.svc.cluster.local`
* `myrabbit-server-1.myrabbit-nodes.mynamespace.svc.cluster.local`
* `myrabbit-server-2.myrabbit-nodes.mynamespace.svc.cluster.local`
* `myrabbit.mynamespace.svc.cluster.local`

Note that the last SAN attribute is the client service DNS name.
Depending on the service type you use (`spec.service.type`), you might need further SAN attributes.
For example if you use service type `NodePort`, you need to include the external IP address of each K8s node to the SAN.
