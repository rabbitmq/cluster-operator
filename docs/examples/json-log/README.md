# JSON Log Example

RabbitMQ 3.9 switched from [Lager](https://github.com/erlang-lager/lager) to the new Erlang [Logger API](https://erlang.org/doc/man/logger.html) supporting JSON-formatted messages.

Pull Requests:
* [Switch from Lager to the new Erlang Logger API for logging #2861](https://github.com/rabbitmq/rabbitmq-server/pull/2861)
* [Logging: Add configuration variables to set various formats #2927](https://github.com/rabbitmq/rabbitmq-server/pull/2927)

This example (requiring RabbitMQ >= v3.9.3) configures RabbitMQ to output JSON logs.

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```

And once deployed, you can see the JSON logs:

```shell
kubectl logs json-server-0 -c rabbitmq
```
