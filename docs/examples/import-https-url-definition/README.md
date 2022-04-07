# Import Definitions Example

You can import definitions, which contain definitions of all broker objects from a URL accessible over HTTPS. [Learn more about export and import rabbitmq definitions](https://www.rabbitmq.com/definitions.html#import).

This is useful also for very large definitions (thousands of queues) when the config map maximum size of 1Mb is not enough.

You can rely on these RabbitMQ configuration parameters:

```
definitions.import_backend = https
definitions.https.url = https://raw.githubusercontent.com/rabbitmq/sample-configs/main/lot-of-queues/5k-queues.json
definitions.tls.versions.1 = tlsv1.2
```

Check out `rabbitmq.yaml` as an example.

Importing a very large definition cluster with several thousands of queues like in the example takes a good amount of memory and in general computation resources.
For the example let's run it with a good amount of memory like 8GB and at least 2 cpu to be sure.

Keep in mind that exported definitions contain all broker objects, including users. This means that the default-user credentials will be imported from the definitions, and will not be the one which is generated at the creation of the deployment as a kubernetes secret object.



