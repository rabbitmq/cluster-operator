# Mutual TLS Peer Verification (Mutual TLS Authentication, mTLS) Example

This example is an extension of the [basic TLS example](../tls) and 
adds mutual peer verification ("mTLS") in addition to enabling TLS for client connections.

It is recommended to get familiar at least with the basics of [TLS setup in RabbitMQ](https://www.rabbitmq.com/ssl.html)
before going over this example, in particular with [how TLS peer verification works](https://www.rabbitmq.com/ssl.html#peer-verification).

TLS has multiple moving parts and concepts involved. Successful TLS connections
require sufficient and compatible configuration on **both server and client sides**, and understanding the terminology
used would help a lot.


## Specifying Server Certificate and Key

Both RabbitMQ and clients can [verify each other's certificate chain](https://www.rabbitmq.com/ssl.html#peer-verification) for
trust. When such verification is performed on both ends, the practice is sometimes
referred to "mutual TLS authentication" or simply "mTLS". This example
focuses on enabling mutual peer verifications for client connections (as opposed to [node-to-node communication](../mtls-inter-node)).

For clients to perform peer verification of RabbitMQ nodes, they must be provided the necessary TLS [certificates and private keys](https://www.rabbitmq.com/ssl.html#certificates-and-keys) in Secret objects.
It is necessary to set `.spec.tls.secretName` to the name of a secret containing the RabbitMQ server's TLS certificate and key,
In addition, set `spec.tls.caSecretName` to the name of a secret containing the certificate of the [Certificate Authority](https://www.rabbitmq.com/ssl.html#certificates-and-keys) which
has signed the certificates of your RabbitMQ clients.

First, you need to create the Secret which will contain the public certificate and private key to be used for TLS on the RabbitMQ nodes.
Assuming you already have these created and accessible as `server.pem` and `server-key.pem`, respectively, this Secret can be created by running:

```shell
kubectl create secret tls tls-secret --cert=server.pem --key=server-key.pem
```

In order for peer verification to work, the RabbitMQ nodes must trust a Certificate Authority which has signed
the public certificates of any clients which try to connect.

You must create a Secret containing the CA's public certificate so that the RabbitMQ nodes know to trust any certificates signed by the CA.
Assuming the CA's certificate is accessible as `ca.pem`, you can create this Secret by running:

```shell
kubectl create secret generic ca-secret --from-file=ca.crt=ca.pem
```

The Secret must be stored with key 'ca.crt'. These Secrets can also be created by [Cert Manager](https://cert-manager.io/).

Once the secrets exist, you can deploy this example as follows:

```shell
kubectl apply -f rabbitmq.yaml
```

## Enabling Mutual Peer Verification ("mTLS")

With client peer verification enabled, any clients that attempt to connect to the RabbitMQ server that present a TLS certificate will have that
certificate verified against the CA certificate provided in `spec.tls.caSecretName`. This is because the RabbitMQ configuration option
`ssl_options.verify_peer` is enabled with mTLS by default.

To reject client connections that do not present a certificate, enable `ssl.options.fail_if_no_peer_cert`,
this can be done by editing the RabbitmqCluster object's spec to include the field in the `additionalConfig`:

```yaml
spec:
  rabbitmq:
    additionalConfig: |
      ssl_options.fail_if_no_peer_cert = true
```


## Troubleshooting

RabbitMQ has a guide that explains a methodology for [troubleshooting TLS](https://www.rabbitmq.com/troubleshooting-ssl.html) using
OpenSSL command line tools. This methodology helps narrow down connectivity issues quicker.

In the context of Kubernetes, OpenSSL CLI tools can be run on RabbitMQ nodes using `kubectl exec`, e.g.:

``` shell
kubectl exec -it tls-server-0 -- openssl s_client -connect tls-nodes.examples.svc.cluster.local:5671 </dev/null
```
