# Federation Over TLS Example

This is a more complex example of deploying a `RabbitmqCluster` and setting up federation between two virtual hosts. The
cluster has TLS enabled and therefore federation works over a TLS connection.

First, please follow [TLS example](../tls) to create a TLS secret. Alternatively, if you have
[cert-manager](https://cert-manager.io/docs/installation/kubernetes/), you can apply the certificate `certificate.yaml` file.
The certificate expects to have a `ClusterIssuer` named `selfsigned-issuer`. Feel free to adapt this value accordingly to your
cert-manager installation.

**NOTE** `certificate.yaml` contains the word "examples" multiple times - in the `namespace` and `dnsNames` properties.
You need to replace all occurrences with your desired namespace. `dnsNames` values need to contain the actual namespace name this cluster will be deployed to, otherwise TLS will fail due to hostname mismatch.

In addition, you have to create a ConfigMap to import the definitions with the topology pre-defined.

```bash
kubectl apply -f certificate.yaml
kubectl create configmap definitions --from-file=./definitions.json
```

The example has two vhosts "upstream" and "downstream". Both vhosts have a fanout exchange 'example', bound to quorum queue 'qq2'
in "upstream", quorum queue 'qq1' and classic queue 'cq1' in "downstream". There is a policy in the "downstream" to federate
the exchange 'example'. All messages published to 'example' exchange in "upstream" will be federated/copied to 'example' exchange
in "downstream", where the bindings will be applied.

The definitions also import two users: `admin` and `federation`, with passwords matching the usernames (e.g. admin/admin). Note that
due to the imported definitions, the credentials created by the Operator in Secret `federation-default-user` won't be applied/effective.

If you don't want to import the definitions, or want to manually create the topology, the file `rabbitmq-without-import.yaml` will
create a RabbitMQ single-node, with federation plugins enabled and TLS configured.

Learn [more about RabbitMQ Federation](https://www.rabbitmq.com/federation.html).
