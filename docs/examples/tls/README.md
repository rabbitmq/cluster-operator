# TLS Example

You can enable TLS by setting `.spec.tls` to the name of a secret containing TLS certificate and key.

First, you need to create a secret like this (assuming you already have `server.pem` and `server-key.pem` files):

```shell
kubectl create secret tls tls-secret --cert=server.pem --key=server-key.pem
```

This secret can also be created by [Cert Manager](https://cert-manager.io/).

Once the secret exists, you can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```
