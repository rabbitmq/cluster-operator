# TLS Example

You can enable TLS by setting `.spec.tls.secretName` to the name of a secret containing TLS certificate and key.

First, you need to create the Secret which will contain the public certificate and private key to be used for TLS on the RabbitMQ nodes.
Assuming you already have these created and accessible as `server.pem` and `server-key.pem`, respectively, this Secret can be created by running:

```shell
kubectl create secret tls tls-secret --cert=server.pem --key=server-key.pem
```

This secret can also be created by [Cert Manager](https://cert-manager.io/).

Once the secret exists, you can deploy this example as follows:

```shell
kubectl apply -f rabbitmq.yaml
```
