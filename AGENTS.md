This project is a Kubernetes operator for RabbitMQ clusters. It is written in Go and uses the Kubernetes API to manage the lifecycle of RabbitMQ clusters.

# Tests

Tests are written using Ginkgo and Gomega. Tests are separated into three categories:

- Unit tests
- Integration tests
- System tests

## Unit tests

Unit tests are written using Ginkgo and Gomega. They are located in the `api` and `internal` directories.

Run unit tests using Makefile:

```bash
make unit-tests
```

## Integration tests

Integration tests are written using Ginkgo and Gomega. They are located in the `controllers` directory. The controller suite
starts a local Kuberntes API server. It uses `testenv` package to start the server.

Run integration tests using Makefile:

```bash
make integration-tests
```

## System tests

System tests are written using Ginkgo and Gomega. They are located in the `system_tests` directory.

Run system tests using Makefile:

```bash
make system-tests
```

System tests assume that the cluster operator is installed in the cluster. 
System tests do not deploy the cluster operator before running the tests.

# Generation

Code generation is done using `controller-gen`.

Run code generation using Makefile:

```bash
make generate
```

YAML manifests are generated using `controller-gen`.

Run YAML generation using Makefile:

```bash
make manifests
```

# Appendix

## Running on MacOS

MacOS uses an old version of `make`. Try to use `gmake` instead. Fallback to `make` if `gmake` is not available.