# Agent Instructions for RabbitMQ Cluster Operator

This is a Kubernetes operator for RabbitMQ clusters, written in Go using the Kubebuilder framework. It manages the lifecycle (creation, upgrade, graceful shutdown) of RabbitMQ clusters via a `RabbitmqCluster` Custom Resource.

**Trust these instructions.** Only search for additional information if the instructions are incomplete or produce errors.

## Quick Reference

| Action | Command |
|--------|---------|
| Install tools | `gmake install-tools` |
| Run unit tests | `gmake just-unit-tests` |
| Run integration tests | `gmake just-integration-tests` |
| Run all local tests | `gmake just-unit-tests just-integration-tests` |
| Generate code | `gmake generate` |
| Generate CRD/RBAC manifests | `gmake manifests` |
| Lint/format | `gmake checks` |
| Build binary | `gmake manager` |

## Build & Test Commands

**IMPORTANT:** On macOS, always use `gmake` instead of `make`. The system `make` is outdated.

### Prerequisites

- Go 1.25+ (check with `go version`)
- Tools are installed via `gmake install-tools` (runs automatically as dependency of most targets)

### Running Tests Locally

**Unit tests** (fast, ~20 seconds):
```bash
gmake just-unit-tests
```
Tests in: `api/`, `internal/`, `pkg/`

**Integration tests** (~50 seconds, uses envtest with local K8s API server):
```bash
gmake just-integration-tests
```
Tests in: `controllers/`

**Full test targets** (with code generation, formatting, linting):
```bash
gmake unit-tests        # generate + fmt + vet + vuln + unit tests
gmake integration-tests # generate + fmt + vet + vuln + integration tests
```

**System tests** require a running Kubernetes cluster with the operator deployed. Skip these unless explicitly needed.

### Code Generation

**Always run after modifying API types (`api/v1beta1/*.go`) or controller RBAC markers:**
```bash
gmake generate   # Generates DeepCopy methods and API reference docs
gmake manifests  # Generates CRD and RBAC YAML in config/
```

### Linting & Formatting

```bash
gmake checks  # Runs: go fmt, go vet, govulncheck
```

No external linter configuration exists. Standard Go tooling is used.

## Project Layout

```
├── api/v1beta1/              # CRD types (RabbitmqCluster)
│   └── rabbitmqcluster_types.go  # Main type definitions
├── controllers/              # Reconciliation logic
│   ├── rabbitmqcluster_controller.go  # Main controller
│   ├── reconcile_*.go        # Reconciliation sub-functions
│   └── suite_test.go         # Integration test setup (envtest)
├── internal/
│   ├── resource/             # Kubernetes resource builders (StatefulSet, Services, etc.)
│   ├── status/               # Status condition helpers
│   ├── metadata/             # Labels/annotations helpers
│   └── scaling/              # Scale-down logic
├── config/
│   ├── crd/bases/            # Generated CRD YAML
│   ├── rbac/                 # Generated RBAC YAML
│   └── manager/              # Deployment manifests
├── main.go                   # Operator entry point
├── Makefile                  # Build targets (use gmake on macOS)
└── system_tests/             # End-to-end tests (require cluster)
```

### Key Files to Modify

| Change Type | Files to Modify | Post-Change Commands |
|-------------|-----------------|---------------------|
| API fields | `api/v1beta1/rabbitmqcluster_types.go` | `gmake generate manifests` |
| Controller logic | `controllers/rabbitmqcluster_controller.go`, `controllers/reconcile_*.go` | `gmake just-integration-tests` |
| Resource generation | `internal/resource/*.go` | `gmake just-unit-tests` |
| RBAC permissions | Add markers in `controllers/*.go` | `gmake manifests` |

## CI/CD Pipeline

The GitHub Actions workflow (`.github/workflows/build-test-publish.yml`) runs:

1. **Unit + Integration tests** (`gmake install-tools kubebuilder-assets just-unit-tests`, then `gmake just-integration-tests`)
2. **Build operator image**
3. **System tests** against KinD cluster
4. **kubectl plugin tests**
5. **Documentation example tests**

### Replicating CI Checks Locally

Before submitting changes, run:
```bash
gmake checks              # fmt, vet, govulncheck
gmake just-unit-tests     # Unit tests
gmake just-integration-tests  # Integration tests
```

## Testing Framework

Tests use **Ginkgo v2** and **Gomega**. Test files follow the pattern `*_test.go`.

- Integration tests (`controllers/`) use `envtest` which spins up a local kube-apiserver
- The envtest binaries are downloaded automatically to `testbin/`

### Running Specific Tests

```bash
# Run tests matching a pattern
gmake just-unit-tests GINKGO_EXTRA="--focus 'StatefulSet'"

# Run a single test file's package
go run github.com/onsi/ginkgo/v2/ginkgo -v ./internal/resource/
```

## Common Issues & Solutions

| Problem | Solution |
|---------|----------|
| `make: *** No rule to make target` | Use `gmake` instead of `make` on macOS |
| Test failures about missing kubebuilder assets | Run `gmake kubebuilder-assets` first |
| CRD changes not reflected | Run `gmake manifests` after modifying API types |
| DeepCopy methods missing | Run `gmake generate` after modifying API types |
| Import errors after changing types | Run `go mod tidy` |

## Code Conventions

- Follow [Kubernetes Go conventions](https://github.com/kubernetes/community/blob/master/contributors/guide/coding-conventions.md)
- Kubebuilder markers are used for CRD generation and RBAC (see `+kubebuilder:` comments)
- Controller-runtime patterns for reconciliation

## Environment Variables (for system tests only)

- `NAMESPACE`: Namespace for test resources
- `K8S_OPERATOR_NAMESPACE`: Namespace where operator is deployed (default: `rabbitmq-system`)
- `KUBEBUILDER_ASSETS`: Path to envtest binaries (set automatically by Makefile)
