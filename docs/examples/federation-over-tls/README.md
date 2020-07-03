# Federation Over TLS Example

This is the a more complex example of deploying two `RabbitmqCluster`s and setting up federation between them. Upstream cluster has TLS enabled and therefore federation works over a TLS connection.

First, please follow [TLS example](../tls) to create a TLS secret. Once you have a secret, run the `setup.sh` script:

```shell
./setup.sh
```

The script will stop at some point and ask you to run `sudo kubefwd svc`. This is so that `rabbitmqadmin` can connect to the Management API and configure federation.

Therefore to use this script as-is, you need both [kubefwd](https://github.com/txn2/kubefwd) and [rabbitmqadmin](https://www.rabbitmq.com/management-cli.html) CLIs on your machine.

Learn [more about RabbitMQ Federation](https://www.rabbitmq.com/federation.html).
