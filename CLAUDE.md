# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kubernetes operator for deploying and managing RabbitMQ clusters. Built with **Kubebuilder v4** (single-group layout). Module: `github.com/rabbitmq/cluster-operator/v2`, Go 1.26.4.

The operator defines the `RabbitmqCluster` CRD and implements a reconciliation controller managing the full lifecycle: creation, scaling, updates, and graceful shutdown of RabbitMQ clusters on Kubernetes.

See `AGENTS.md` for detailed Kubebuilder patterns, markers reference, and controller design rules.

## Key Commands

```bash
# Code generation — run after editing *_types.go or kubebuilder markers
make manifests      # CRDs + RBAC from markers
make generate       # DeepCopy methods

# Code quality
make fmt vet        # Format and vet
make checks         # fmt + vet + govulncheck

# Tests
make just-unit-tests          # Ginkgo only (no regen), api/ internal/ pkg/ excluding integration label
make just-integration-tests   # Ginkgo with integration label on internal/controller/ (needs kubebuilder-assets)
make unit-tests               # Full: regen + just-unit-tests
make integration-tests        # Full: regen + just-integration-tests
make system-tests             # Against live cluster (~/.kube/config required)
make test-e2e                 # Creates Kind cluster, runs e2e, destroys cluster
make kubectl-plugin-tests     # Bats tests for kubectl plugin (requires ./bin/kubectl-rabbitmq.bats)

# Run a single focused test
make just-unit-tests GINKGO_EXTRA="-v --focus='TestName'"

# Local run against configured cluster
make run OPERATOR_NAMESPACE=rabbitmq-system
make just-run OPERATOR_ARGS="--zap-log-level=debug"

# Build & deploy
make manager                  # Build binary
make docker-build docker-push IMG=<registry>/<image>:<tag>
make deploy IMG=<registry>/<image>:<tag>
make install-tools            # Download all tool binaries to ./bin
```

## Architecture

### Entry Point

`cmd/main.go` initializes the controller-runtime manager, registers `RabbitmqClusterReconciler` and `DeprecatedFeatureReconciler`, and starts leader election.

### API (`api/v1beta1/`)

- `rabbitmqcluster_types.go` — CRD schema with kubebuilder validation markers. Key spec fields: `replicas`, `image`, `persistence`, `resources`, `rabbitmq` (plugins/env/advanced config), `tls`, `override` (raw manifest overrides), `terminationGracePeriodSeconds` (default 1 week).
- `zz_generated.deepcopy.go` — **DO NOT EDIT** (from `make generate`)

### Main Controller (`internal/controller/`)

`rabbitmqcluster_controller.go` drives the reconciliation loop. Steps are split into focused files:

| File | Purpose |
|------|---------|
| `reconcile_operator_defaults.go` | Default image/namespace |
| `reconcile_tls.go` | TLS certificate validation |
| `reconcile_cli.go` | RabbitMQ CLI commands (feature flags, user setup) |
| `reconcile_status.go` | Status subresource updates |
| `reconcile_scale_down.go` / `reconcile_scale_zero.go` | Coordinated scale-down with queue draining |
| `reconcile_persistence.go` | PVC management |
| `reconcile_finalizer.go` | Cleanup on CR deletion |

`deprecated_feature_controller.go` — Runs every 5 minutes, checks live clusters for deprecated features via management API. Disable with `DISABLE_DEPRECATED_FEATURES_CHECK=true`.

### Resource Builders (`internal/resource/`)

Each file builds/updates one Kubernetes resource type (StatefulSet, ConfigMap, Services, Secrets, RBAC). Builders are idempotent and support `override` specs from the CR.

### Supporting Packages

- `internal/rabbitmqclient/` — RabbitMQ HTTP management API client (wraps `michaelklishin/rabbit-hole`)
- `internal/scaling/` — Queue draining coordination for scale-down
- `internal/status/` — Cluster state queries
- `internal/metadata/` — Pod/resource metadata helpers

### kubectl Plugin (`kubectl-rabbitmq/`)

Separate Go module with its own `go.mod`. Tests live in `kubectl-rabbitmq/*_test.go` and run with `go test` (see `kubectl-rabbitmq/Makefile`).

## Testing

- **Unit tests**: `api/v1beta1/*_test.go`, `internal/resource/*_test.go` — Ginkgo/Gomega, no `integration` label
- **Integration tests**: `internal/controller/*_test.go` — uses Envtest (embedded etcd + kube-apiserver), labeled `integration`. `suite_test.go` manages env setup.
- **System tests**: `test/system/` — Ginkgo against a live cluster
- **E2E tests**: `test/e2e/` — Kind cluster + cert-manager, built with `-tags=e2e`. `test/utils/utils.go` provides shared Go helpers for the E2E suite.

Envtest binaries are downloaded to `testbin/`. If asset setup fails: `make clean-testbin` then retry.

## Operator Environment Variables

| Var | Default | Purpose |
|-----|---------|---------|
| `OPERATOR_NAMESPACE` | (required) | Namespace where operator runs |
| `OPERATOR_SCOPE_NAMESPACE` | (all) | Comma-separated namespaces to watch |
| `DEFAULT_RABBITMQ_IMAGE` | `rabbitmq:4.2.6-management` | Default RabbitMQ image |
| `DISABLE_DEPRECATED_FEATURES_CHECK` | enabled | Set to disable deprecation checks |
| `ENABLE_DEBUG_PPROF` | false | Enable pprof endpoints |

## Auto-Generated Files (Never Edit)

- `api/v1beta1/zz_generated.deepcopy.go` — `make generate`
- `config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml` — `make manifests`
- `config/rbac/role.yaml` — `make manifests`
- `PROJECT` — Kubebuilder metadata

Never remove `// +kubebuilder:scaffold:*` markers — the CLI injects code at these locations.

## Releases

Push a `v*` tag to trigger the GitHub Actions release pipeline: it builds the image, runs all tests, pushes to registries, and creates a draft release with installation manifests.
