# TLS Example

This example demonstrates how to [enable TLS in RabbitMQ](https://www.rabbitmq.com/ssl.html) deployed on Kubernetes
with this Operator. It is accompanied by a more advanced [example that enabled mutual peer verification](../mtls) ("mTLS")
for client connections.

It is recommended to get familiar at least with the basics of [TLS setup in RabbitMQ](https://www.rabbitmq.com/ssl.html)
before going over this example. TLS has multiple moving parts and concepts involved. Successful TLS connections
require sufficient and compatible configuration on **both server and client sides**, and understanding the terminology
used would help a lot.


## Specifying Server Certificate and Key

Setting `.spec.tls.secretName` to the name of a secret containing [server certificate and key](https://www.rabbitmq.com/ssl.html#certificates-and-keys)
will enable TLS for the deployed nodes.

As a first step, create a Secret which will contain the public certificate and private key to be used for TLS on the RabbitMQ nodes.

This example assumes you have obtained (or [generated a self-signed](https://github.com/michaelklishin/tls-gen)) a server certificate/key pair
accessible as `server.pem` and `server-key.pem`, respectively. Create a Secret by running:

```shell
kubectl create secret tls tls-secret --cert=server.pem --key=server-key.pem
```

Alternatively, this secret can also be created by [Cert Manager](https://cert-manager.io/).

Once the secret exists, you can deploy this example as follows:

```shell
kubectl apply -f rabbitmq.yaml
```

## Subject Alternative Names (SANs) Attributes for Certificates

Subject Alternative Names (SANs) is a set of certificate fields used to identify a server or client.
It is important for the certificates used by RabbitMQ nodes to use identifiers that client
would expect (trust).

Make sure that the certificate's Subject Alternative Name (SAN) contains at least the following attributes:

* `*.<RabbitMQ cluster name>-nodes.<namespace>.svc.<K8s cluster domain name>`
* `<RabbitMQ cluster name>.<namespace>.svc.<K8s cluster domain name>`

If wildcards are not permitted, a **separate SAN attribute** must be present for every RabbitMQ node in the cluster.

For example, with a 3-node RabbitMQ cluster named `myrabbit` in namespace `mynamespace` with the default Kubernetes cluster domain `cluster.local`,
the SANs must include at least the following attributes:

* `myrabbit-server-0.myrabbit-nodes.mynamespace.svc.cluster.local`
* `myrabbit-server-1.myrabbit-nodes.mynamespace.svc.cluster.local`
* `myrabbit-server-2.myrabbit-nodes.mynamespace.svc.cluster.local`
* `myrabbit.mynamespace.svc.cluster.local`

Note that the last SAN attribute is the client service DNS name.
Depending on the service type you use (`spec.service.type`), you might need further SAN attributes.
For example if you use service type `NodePort`, you need to include the external IP address of each K8s node to the SAN.


## Troubleshooting

RabbitMQ has a guide that explains a methodology for [troubleshooting TLS](https://www.rabbitmq.com/troubleshooting-ssl.html) using
OpenSSL command line tools. This methodology helps narrow down connectivity issues quicker.

In the context of Kubernetes, OpenSSL CLI tools can be run on RabbitMQ nodes using `kubectl exec`, e.g.:

``` shell
kubectl exec -it tls-server-0 -- openssl s_client -connect tls-nodes.examples.svc.cluster.local:5671 </dev/null
```
