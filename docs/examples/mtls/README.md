# mTLS Example

You can enable mTLS by providing the necessary TLS certificates and keys in Secret objects.
You must set `.spec.tls.secretName` to the name of a secret containing the RabbitMQ server's TLS certificate and key,
and set `spec.tls.caSecretName` to the name of a secret containing the certificate of the Certificate Authority which
has signed the certificates of your RabbitMQ clients.

First, you need to create the Secret which will contain the public certificate and private key to be used for TLS on the RabbitMQ nodes.
Assuming you already have these created and accessible as `server.pem` and `server-key.pem`, respectively, this Secret can be created by running:

```shell
kubectl create secret tls tls-secret --cert=server.pem --key=server-key.pem
```

In order to use mTLS, the RabbitMQ nodes must trust a Certificate Authority which has signed the public certificates of any clients which try to connect.
You must create a Secret containing the CA's public certificate so that the RabbitMQ nodes know to trust any certifiates signed by the CA.
Assuming the CA's certificate is accessible as `ca.pem`, you can create this Secret by running:

```shell
kubectl create secret generic ca-secret --from-file=ca.crt=ca.pem
```

These Secrets can also be created by [Cert Manager](https://cert-manager.io/).

Once the secrets exist, you can deploy this example as follows:

```shell
kubectl apply -f rabbitmq.yaml
```

## Peer verification

With mTLS enabled, any clients that attempt to connect to the RabbitMQ server that present a TLS certificate will have that
certificate verified against the CA certificate provided in `spec.tls.caSecretName`. This is because the RabbitMQ configuration option
`ssl_options.verify_peer` is enabled with mTLS by default.

If you require RabbitMQ to reject clients that do not present certificates by enabling `ssl.options.fail_if_no_peer_cert`,
this can be done by editing the RabbitmqCluster object's spec to include the field in the `additionalConfig`:

```yaml
spec:
  rabbitmq:
    additionalConfig: |
      ssl_options.fail_if_no_peer_cert = true
```

