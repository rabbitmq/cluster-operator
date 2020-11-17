# mtls-inter-node Example

This example shows how to [secure the Erlang Distribution with TLS](https://www.rabbitmq.com/clustering-ssl.html) so that RabbitMQ cluster nodes communicate over secure channels.
In the future, the RabbitMQ Cluster Operator may make this easier to configure but it is already possible with the [`envConfig`](https://www.rabbitmq.com/kubernetes/operator/using-operator.html#env-config) and [`override`](https://www.rabbitmq.com/kubernetes/operator/using-operator.html#override) properties.

The most important parts of this example are:

* `rabbitmq.yaml` - `RabbitmqCluster` definition with all the necessary configuration
* `inter_node_tls.config` - Erlang Distribution configuration file that will be mounted as a volume

The other files serve as an example for setting up certificates with Cert Manager.

* `rabbitmq-ca.yaml` - defines an `Issuer` (CA)
* `rabbitmq-certificate.yaml` - defines a certificate that will be provisioned by Cert Manager and then mounted as a volume

`setup.sh` should perform all the necessary steps but may need to be adjusted to work on your system.

```shell
# install Cert Manager
kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.4/cert-manager.yaml
# deploy the example
./setup.sh
```

To validate that RabbitMQ nodes connect over TLS you can run the following checks:

```shell
# check that the distribution port has TLS enabled (this command should return `Verification: OK`)
kubectl exec -it mtls-inter-node-server-0 -- bash -c 'openssl s_client -connect ${HOSTNAME}${K8S_HOSTNAME_SUFFIX}:25672 -state -cert /etc/rabbitmq/certs/tls.crt  -key /etc/rabbitmq/certs/tls.key -CAfile /etc/rabbitmq/certs/ca.crt 2>&1 | grep Verification'

# check that distribution uses TLS (this command should return `{ok,[["inet_tls"]]}`)
kubectl exec -it mtls-inter-node-server-0 -- rabbitmqctl eval 'init:get_argument(proto_dist).'
```
