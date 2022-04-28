# HashiCorp Vault Default User Example

The RabbitMQ Cluster Operator supports storing RabbitMQ default user (admin) credentials and RabbitMQ server certificates in
[HashiCorp Vault](https://www.vaultproject.io/).

As explained in [this KubeCon talk](https://youtu.be/w0k7MI6sCJg?t=177) there four different approaches in Kubernetes to consume external secrets:
1. Direct API
2. Controller to mirrors secrets in K8s
3. Sidecar + MutatingWebhookConfiguration
4. Secrets Store CSI Driver

This example takes the 3rd approach (`Sidecar + MutatingWebhookConfiguration`) integrating with Vault using [vault-k8s](https://github.com/hashicorp/vault-k8s). If `spec.secretBackend.vault.defaultUserPath` is set in the RabbimqCluster CRD, the Cluster Operator will **not** create a K8s Secret for the default user credentials. Instead, Vault init and sidecar containers will fetch username and password from Vault.

If `spec.secretBackend.vault.tls.pkiIssuerPath` is set, short-lived server certificates are issued from [Vault PKI Secrets Engine](https://www.vaultproject.io/docs/secrets/pki) upon every RabbitMQ Pod (re)start. See [examples/vault-tls](../vault-tls) for more information.

(This Vault integration example is independent of and not to be confused with the [Vault RabbitMQ Secrets Engine](https://www.vaultproject.io/docs/secrets/rabbitmq).)

This example is presented in [this](https://youtu.be/twCzPEAjy8M) video.

## Usage

This example requires:
1. Vault server is installed.
2. [Vault agent injector](https://www.vaultproject.io/docs/platform/k8s/injector) is installed.
3. [Vault Kubernetes Auth Method](https://www.vaultproject.io/docs/auth/kubernetes) is enabled.
4. The RabbitMQ admin credentials were already written to Vault to path `spec.secretBackend.vault.defaultUserPath` with keys `username` and `password` (by some cluster-operator external mechanism. The cluster-operator will never write admin credentials to Vault).
5. Role `spec.secretBackend.vault.role` is configured in Vault with a policy to read from `defaultUserPath`.

Run script [setup.sh](./setup.sh) to get started with a Vault server in [dev mode](https://www.vaultproject.io/docs/concepts/dev-server) fulfilling above requirements.
[setup.sh](./setup.sh) assumes you are using namespace `examples`, which can be created by:

```shell
kubectl create ns examples
```

If you want to deploy this example in a different existing namespace, you can set environment variable `RABBITMQ_NAMESPACE` when you run the script.
(This script is not production-ready. It is only meant to get you started experiencing end-to-end how RabbitMQ integrates with Vault.)

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```

And once deployed, you can check that the admin user credentials got provisioned by Vault:

```shell
kubectl exec vault-default-user-server-0 -c rabbitmq -- rabbitmqctl authenticate_user <username> <password>
```
where `<username>` and `<password>` are the values from step 4 above.

## Admin password rotation without restarting Pods
Rotating the admin password (but not the username!) is supported without the need to restart RabbitMQ servers.

This is how it works:
1. The RabbitMQ cluster operator deploys a sidecar container called `default-user-credential-updater`
2. When the default user password changes in Vault server, the Vault sidecar container writes the new admin password to file `/etc/rabbitmq/conf.d/11-default_user.conf`
3. The `default-user-credential-updater` sidecar watches file `/etc/rabbitmq/conf.d/11-default_user.conf` and when it changes, it updates the password in RabbitMQ.
4. Additionally, the sidecar updates the local file `/var/lib/rabbitmq/.rabbitmqadmin.conf` with the new password (required by `rabbitmqadmin` CLI).

The `default-user-credential-updater` image can be overriden:
```
   vault:
      defaultUserUpdaterImage: "rabbitmqoperator/default-user-credential-updater:1.0.1"
      ...
```

To disable the sidecar container, set the image to an empty string:
```
   vault:
      defaultUserUpdaterImage: ""
      ...
```
