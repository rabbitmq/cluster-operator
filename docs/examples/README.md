## RabbitMQ Cluster Operator examples

This section contains examples on how to configure some features of RabbitMQ.
The examples are common use cases e.g. [tls](./tls) to configure specific RabbitMQ features
or how RabbitMQ Pods will behave inside Kubernetes.

### Testing framework

Some examples have tests to ensure that the example has achieved its intention. Any new examples
should have tests. Exceptions apply if the feature itself is sufficiently tested in the code, for
example, [resource limits](./resource-limits) is tested in the code to ensure that given a set of
inputs, the Pod resource requests are configured accordingly. Duplicating the same assertion here
would not make sense, and `exec`'ing into the container to ensure that Kubernetes has respected
the resource requests would fall under Kubernetes Core testing.

### Writing tests for examples

Every folder with an example must have a file `test.sh`. This executable Bash file has to assert on
the state of RabbitMQ to ensure that it has been configured according to the expectations of the example.

If the test requires some preparation or setup, a file `setup.sh` can be provided to be executed
**before** the RabbitMQ cluster is created. This file must be a Bash executable. This file can assume
that `kubectl` is configured to execute commands against a working Kubernetes cluster.

The script `test.sh` will be executed **after** `AllReplicasReady` condition is `True` in `RabbitmqCluster`
object. The script `test.sh` should exit with code 0 is all assertions were successful; the script `test.sh` should
exit with non-zero code if any test or assertion failed. The same is expected for `setup.sh`.

If the example should not run any tests because of the reasons mentioned above, the folder should contain
a file `.ci-skip`, so that the example is not considered in our tests.

Once the `test.sh` script has completed, the namespace where the example was applied will be deleted. This allows
to have a clean slate for the next test to execute. This also means that `test.sh` does not need to
tear down namespaced resources.

The test and setup scripts can assume that [Cert Manager](https://cert-manager.io/) is installed and available.
There is also a cluster issuer to produce self-signed certificates, named `selfsigned-issuer`. It is also
acceptable to create local `Issuer`s when needed.

