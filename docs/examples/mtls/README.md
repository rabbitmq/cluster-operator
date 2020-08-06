# mTLS Example

You can enable mTLS by providing the necessary TLS certificates and keys in Secret objects.
You must set `.spec.tls.secretName` to the name of a secret containing the RabbitMQ server's TLS certificate and key,
and set `spec.tls.caSecretName` to the name of a secret containing the certificate of the Certificate Authority which
has signed the certificates of your RabbitMQ clients.

First, you need to create the server Secret like this (assuming you already have `server.pem` and `server-key.pem` files):

```shell
kubectl create secret tls tls-secret --cert=server.pem --key=server-key.pem
```

Next, you must create the Secret containing the CA's certificate like this (assuming you already have a `ca.pem` file):

```shell
kubectl create secret generic ca-secret --from-file=ca.crt=ca.pem
```

These Secrets can also be created by [Cert Manager](https://cert-manager.io/).

Once the secrets exist, you can deploy this example like this:

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

