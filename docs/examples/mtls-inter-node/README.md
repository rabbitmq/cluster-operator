# mtls-inter-node Example

This example shows how to secure Erlang Distribution with TLS so that RabbitMQ cluster nodes communicate over secure channels.
In future, RabbitMQ Cluster Operator may make it easier to configure but it is already possible to achieve that with `envConfig` and `override` properties.

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
