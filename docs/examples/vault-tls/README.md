# HashiCorp Vault TLS Example

As described in the [vault-default-user example](../vault-default-user), you can configure RabbitmqClusters to have their server certificates issued by [Vault PKI Secrets Engine](https://www.vaultproject.io/docs/secrets/pki). This is in contrast to the [TLS example](../tls) where certificate and private key are put into a Kubernetes Secret object before the RabbitmqCluster is created.

If `spec.secretBackend.vault.tls.pkiIssuerPath` is set, new short-lived certificates are issued from Vault PKI Secrets Engine upon every RabbitMQ Pod (re)start aligning with Vault's philosophy of short-lived secrets.
The private key is never stored in Vault.
Before the short-lived server certificate expires, the Vault sidecar container will request new a certificate putting it into `/etc/rabbitmq-tls/` where it will be picked up on-the-fly by the Erlang VM without the need to restart the pod.

This example is presented in [this](https://youtu.be/twCzPEAjy8M) video.

## Usage

This example requires:
1. Vault server is installed.
2. [Vault agent injector](https://www.vaultproject.io/docs/platform/k8s/injector) is installed.
3. [Vault Kubernetes Auth Method](https://www.vaultproject.io/docs/auth/kubernetes) is enabled.
4. Vault PKI Secrets Engine [set up](https://www.vaultproject.io/docs/secrets/pki#setup).
5. Role `spec.secretBackend.vault.role` is configured in Vault with a policy to create and update `spec.secretBackend.vault.tls.pkiIssuerPath`.

Run script [setup.sh](./setup.sh) to get started with a Vault server in [dev mode](https://www.vaultproject.io/docs/concepts/dev-server) fullfilling above requirements. (This script is not production-ready. It is only meant to get you started experiencing end-to-end how RabbitMQ integrates with Vault.)

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```

And once deployed, you can check check that TLS is enabled by:

```shell
  kubectl exec vault-tls-server-0 -c rabbitmq -- openssl s_client \
      -connect "vault-tls.examples.svc.cluster.local:5671" \
      -CAfile /etc/rabbitmq-tls/ca.crt \
      -verify_return_error
```
