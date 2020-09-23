# How to Contribute

We'd love to accept your patches and contributions to this project. There are
just a few small guidelines you need to follow.

## GitHub issues

RabbitMQ Cluster Kubernetes Operator team uses GitHub issues for feature development and bug tracking.
The issues have specific information as to what the feature should do and what problem or
use case is trying to resolve. Bug reports have a description of the actual behaviour and
the expected behaviour, along with repro steps when possible. It is important to provide
repro when possible, as it speeds up the triage and potential fix.

We do not use GitHub issues for questions or support requests. For that purpose, it is better
to use [RabbitMQ mailing list][rmq-users] or [RabbitMQ Slack #kubernetes channel][rabbitmq-slack].

For support questions, we strongly encourage you to provide a way to
reproduce the behavior you're observing, or at least sharing as much
relevant information as possible on the [RabbitMQ users mailing
list][rmq-users]. This would include YAML manifests, Kubernetes version,
RabbitMQ Operator logs and any other relevant information that might help
to diagnose the problem.

## Makefile

This project contains a Makefile to perform common development operation. If you want to build, test or deploy a local copy of the repository, keep reading.

### Required environment variables

The following environment variables are required by many of the `make` targets to access a custom-built image:

- DOCKER_REGISTRY_SERVER: URL of docker registry containing the Operator image (e.g. `registry.my-company.com`)
- OPERATOR_IMAGE: path to the Operator image within the registry specified in DOCKER_REGISTRY_SERVER (e.g. `rabbitmq/cluster-operator`). Note: OPERATOR_IMAGE should **not** include a leading slash (`/`)

When running `make deploy-dev`, additionally:

- DOCKER_REGISTRY_USERNAME: Username for accessing the docker registry
- DOCKER_REGISTRY_PASSWORD: Password for accessing the docker registry
- DOCKER_REGISTRY_SECRET: Name of Kubernetes secret in which to store the Docker registry username and password

#### Make targets

- **controller-gen** Download controller-gen if not in $PATH
- **deploy** Deploy operator in the configured Kubernetes cluster in ~/.kube/config
- **deploy-dev** Deploy operator in the configured Kubernetes cluster in ~/.kube/config, with local changes
- **deploy-kind** Load operator image and deploy operator into current KinD cluster
- **deploy-sample** Deploy RabbitmqCluster defined in config/sample/base
- **destroy** Cleanup all operator artefacts
- **kind-prepare** Prepare KinD to support LoadBalancer services, and local-path StorageClass
- **kind-unprepare** Remove KinD support for LoadBalancer services, and local-path StorageClass
- **list** List Makefile targets
- **run** Run operator binary locally against the configured Kubernetes cluster in ~/.kube/config
- **unit-tests** Run unit tests
- **integration-tests** Run integration tests
- **system-tests** Run end-to-end tests against Kubernetes cluster defined in ~/.kube/config

### Testing

Before submitting a pull request, ensure all local tests pass:
- `make unit-tests`
- `make integration-tests`

<!-- TODO: generalise deployment process: make DOCKER_REGISTRY_SECRET and DOCKER_REGISTRY_SERVER configurable -->
Also, run the system tests with your local changes against a Kubernetes cluster:
- `make deploy-dev`
- `make system-tests`

## Pull Requests

RabbitMQ Operator project uses pull requests to discuss, collaborate on and accept code contributions.
Pull requests are the primary place of discussing code changes.

Here's the recommended workflow:

 * [Fork the repository][github-fork] or repositories you plan on contributing to. If multiple
   repositories are involved in addressing the same issue, please use the same branch name
   in each repository
 * Create a branch with a descriptive name
 * Make your changes, run tests (usually with `make unit-tests integration-tests system-tests`), commit with a
   [descriptive message][git-commit-msgs], push to your fork
 * Submit pull requests with an explanation what has been changed and **why**
 * We will get to your pull request within one week. Usually within the next day or two you'll get a response.

If what you are going to work on is a substantial change, please first
ask the core team for their opinion on the [RabbitMQ users mailing list][rmq-users].

### Code Conventions

This project follows the [Kubernetes Code Conventions for Go](https://github.com/kubernetes/community/blob/master/contributors/guide/coding-conventions.md#code-conventions), which in turn mostly refer to [Effective Go](https://golang.org/doc/effective_go.html) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments). Please ensure your pull requests follow these guidelines.

## Code reviews

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.

## Community Guidelines

This project follows [Contributor Covenant](./CODE_OF_CONDUCT.md), version 2.0.

[rmq-users]: https://groups.google.com/forum/#!forum/rabbitmq-users
[git-commit-msgs]: https://chris.beams.io/posts/git-commit/
[github-fork]: https://help.github.com/articles/fork-a-repo/
[rabbitmq-slack]: https://rabbitmq-slack.herokuapp.com/

