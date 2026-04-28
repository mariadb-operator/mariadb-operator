# AGENTS.md

Guidance for AI coding assistants working in `mariadb-operator/mariadb-operator`. Read this file before making changes.

## Project overview

mariadb-operator is a Kubernetes operator for MariaDB built with controller-runtime. It manages the full lifecycle of MariaDB clusters including replication, Galera, backups, restores, and point-in-time recovery. The operator exposes a `k8s.mariadb.com/v1alpha1` API group with CRDs for MariaDB, Backup, PhysicalBackup, Restore, User, Grant, Database, Connection, SQLJob, MaxScale, ExternalMariaDB, and PointInTimeRecovery.

The codebase follows a **phase-based reconciliation** pattern where the main `Reconcile` loop iterates over an ordered list of phase functions. Each phase returns `(ctrl.Result, error)` and short-circuits on non-zero results. Sub-reconcilers in `pkg/controller/` handle child resource reconciliation (StatefulSets, Services, Secrets, ConfigMaps, RBAC, Certificates, etc.).

## Repository layout

- `api/v1alpha1/` -- CRD type definitions, kubebuilder markers, and field indexers
- `cmd/controller/` -- Entry point with cobra-based multi-command (controller, webhook, backup, restore, pitr, init, agent)
- `internal/controller/` -- All Kubernetes controllers (MariaDB, Backup, Restore, User, Grant, Database, Connection, SQLJob, MaxScale, etc.)
- `internal/webhook/v1alpha1/` -- Validation webhooks per CRD
- `pkg/` -- Shared packages:
  - `pkg/builder/` -- Central builder for Kubernetes resources (StatefulSets, Services, Jobs, etc.)
  - `pkg/controller/` -- Sub-reconcilers (secret, configmap, statefulset, service, rbac, auth, deployment, pvc, servicemonitor, certificate, replication, galera, sql, batch)
  - `pkg/sql/` -- SQL client for MariaDB operations (users, grants, databases, replication, Galera)
  - `pkg/condition/` -- Status condition management (Ready, Complete, and MariaDB-specific conditions)
  - `pkg/refresolver/` -- Reference resolver for Kubernetes objects (Secrets, ConfigMaps, MariaDB, MaxScale, etc.)
  - `pkg/watch/` -- WatcherIndexer for indexed watches on external resources
  - `pkg/environment/` -- Operator and pod environment configuration
  - `pkg/discovery/` -- Runtime API discovery for optional features (cert-manager, Prometheus, VolumeSnapshot)
- `config/` -- Kubernetes manifests for operator deployment (CRDs, RBAC, webhooks)
- `deploy/` -- Generated deployment manifests and Helm chart
- `test/e2e/` -- End-to-end tests with Ginkgo/Gomega
- `make/` -- Split Makefile targets (build, dev, deploy, gen, helm, etc.)

## Build and test commands

All targets in the root `Makefile`, which includes modular files under `make/`.

- `make build` -- Build the `bin/mariadb-operator` binary
- `make docker-build` -- Build Docker image
- `make docker-dev` -- Build and load image into KIND cluster
- `make run` -- Run controller locally against a cluster (runs lint first)
- `make webhook` -- Run webhook server locally
- `make test` -- Run unit tests (api, pkg, helmtest, webhook)
- `make test-int` -- Run integration tests against envtest (controller tests)
- `make test-int-basic` -- Run basic integration tests only
- `make lint` -- Run golangci-lint
- `make gen` -- Run all code generation (manifests, deepcopy, embed, helm, docs, examples)
- `make manifests` -- Generate CRDs, RBAC, and webhook manifests via controller-gen
- `make code` -- Generate DeepCopy methods via controller-gen
- `make release` -- Test release locally with goreleaser

Run a single test: `make test TEST_ARGS='-run TestMyTest -v'`
Run a single integration test: `make test-int TEST_ARGS='-run TestMyTest -v'`

## Controller patterns

### Phase-based reconciliation

The main MariaDB controller defines an ordered slice of `reconcilePhaseMariaDB` structs. Each phase has a name and a reconcile function. The loop iterates phases, logs progress, handles errors with status patching, and short-circuits on non-zero results:

```go
phases := []reconcilePhaseMariaDB{
    {Name: "Spec", Reconcile: r.setSpecDefaults},
    {Name: "Status", Reconcile: r.reconcileStatus},
    // ... more phases
}
for _, p := range phases {
    result, err := p.Reconcile(ctx, &mariadb)
    if err != nil {
        if shouldSkipPhase(err) { continue }
        // Patch status with failure, aggregate errors
    }
    if !result.IsZero() { return result, err }
}
```

**When adding a new phase**, append to the slice in the correct logical order. Phase ordering matters: infrastructure (Secrets, ConfigMaps, TLS, RBAC) comes before workloads (StatefulSet, Services), which comes before data operations (Replication, SQL, Restore).

### Sub-reconciler pattern

Sub-reconcilers in `pkg/controller/` handle reconciliation of child Kubernetes resources. They are injected into the main controller struct and follow a consistent pattern:

```go
type StatefulSetReconciler struct {
    Client    client.Client
    Scheme    *runtime.Scheme
    Builder   *builder.Builder
    Discovery *discovery.Discovery
}

func (r *StatefulSetReconciler) Reconcile(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
    sts, err := r.Builder.BuildMariadbStatefulSet(mdb)
    // ... create or patch using server-side apply or client.MergeFrom
}
```

### SQL reconciler pattern (generic)

The `pkg/controller/sql/controller.go` provides a generic reconciler used by User, Grant, Database, and SQLJob controllers. Key interfaces:

- `Resource` -- The CRD being reconciled (MariaDBRef, IsBeingDeleted, RequeueInterval, etc.)
- `WrappedReconciler` -- Resource-specific SQL operations and status patching
- `Finalizer` -- Cleanup logic on deletion

The generic controller handles: deletion finalization, MariaDB reference resolution, health waiting, binlog replay checking, SQL client creation, and requeue with jitter.

### Status patching

Use `client.MergeFrom()` for status patches. The pattern is:

```go
func (r *Reconciler) patchStatus(ctx context.Context, obj *MyCRD, patcher func(*MyCRDStatus) error) error {
    patch := client.MergeFrom(obj.DeepCopy())
    patcher(&obj.Status)
    return r.Status().Patch(ctx, obj, patch)
}
```

### Condition management

Use `pkg/condition` for status conditions. Two main types:
- `Ready` -- For resources with ongoing lifecycle (MariaDB, User, Grant, etc.)
- `Complete` -- For one-time resources (Backup, Restore, SQLJob)

Set conditions using helpers like `conditions.SetReadyHealthy(c)`, `conditions.SetReadyFailed(c)`, `conditions.SetReadyWithStatefulSet(c, sts)`.

### Watch and indexing

Use `pkg/watch.WatcherIndexer` to set up indexed watches on external resources. Each CRD that references external resources implements `IndexerFuncForFieldPath` and registers watches in its `Index*` function (e.g. `api/v1alpha1/mariadb_indexes.go`).

## Webhook patterns

Validation webhooks live in `internal/webhook/v1alpha1/`. Each CRD has a validator that implements `admission.Validator[*Type]`. The pattern:

```go
type MariaDBCustomValidator struct{}
var _ admission.Validator[*v1alpha1.MariaDB] = &MariaDBCustomValidator{}

func (v *MariaDBCustomValidator) ValidateCreate(ctx context.Context, obj *v1alpha1.MariaDB) (admission.Warnings, error) {
    validateFns := []func(*v1alpha1.MariaDB) error{
        validateHA, validateReplication, validateStorage, ...
    }
    for _, fn := range validateFns {
        if err := fn(obj); err != nil { return nil, err }
    }
    return nil, nil
}
```

Immutable fields are enforced via `pkg/webhook/inmutable_webhook.go` using struct tags `webhook:"inmutable"` and `webhook:"inmutableinit"`. Always call the immutable validator first in `ValidateUpdate`.

## SQL client

The `pkg/sql/` package provides a comprehensive MariaDB client. Key operations:
- **Connection**: `NewClient()`, `NewClientWithMariaDB()`, `NewInternalClientWithPodIndex()`, `NewLocalClientWithPodEnv()`
- **Users**: `CreateUser`, `DropUser`, `AlterUser`, `UserExists`
- **Privileges**: `Grant`, `Revoke`
- **Database**: `CreateDatabase`, `DropDatabase`
- **Replication**: `ChangeMaster`, `StartSlave`, `StopAllSlaves`, `ResetAllSlaves`, `WaitForReplicaGtid`
- **Galera**: `GaleraClusterSize`, `GaleraClusterStatus`, `GaleraLocalState`
- **Binary logs**: `ResetMaster`, `BinaryLogIndex`, `GtidBinlogPos`, `GtidCurrentPos`, `SetGtidSlavePos`
- **Locking**: `LockTablesWithReadLock`, `UnlockTables`, `EnableReadOnly`, `DisableReadOnly`

Always use the appropriate client constructor for the context:
- `NewClientWithMariaDB` -- From the controller, connecting to a MariaDB CR
- `NewInternalClientWithPodIndex` -- Connecting to a specific pod by index
- `NewLocalClientWithPodEnv` -- From inside a pod (init containers, sidecars)

## Kubernetes best practices

- **Owner references**: Always set `OwnerReferences` on child resources so they are garbage-collected when the parent is deleted. Use `ctrl.SetControllerReference()`.
- **Finalizers**: Add finalizers for cleanup logic on deletion. The generic SQL reconciler handles this pattern. Use `client.IgnoreNotFound()` when deleting resources in finalizers.
- **Reconcile idempotency**: Every reconcile must be idempotent. Use `Get` + `Create` or `Update` pattern, or server-side apply. Never assume state.
- **Status subresource**: Always patch status separately from spec. Never mutate spec in status updates.
- **Requeue with backoff**: Return `ctrl.Result{RequeueAfter: duration}` for expected wait conditions. Use jitter to avoid thundering herds.
- **Resource limits**: Controllers should set `controller.Options{MaxConcurrentReconciles: n}` to avoid overwhelming the API server.
- **Leader election**: The manager enables leader election by default. Only one controller instance reconciles at a time.
- **RBAC markers**: Use `//+kubebuilder:rbac:` markers above reconciler structs. Run `make manifests` to generate RBAC YAML.

## Database best practices

- **Idempotent SQL**: All SQL operations must be idempotent. Use `CREATE USER IF NOT EXISTS` semantics (check existence first, then create or alter).
- **Connection handling**: Always close SQL connections. The `sql.Client` handles connection lifecycle, but be aware of connection timeouts.
- **Replication safety**: When modifying data on a primary, ensure replicas have caught up using `WaitForReplicaGtid` before proceeding.
- **Galera consensus**: Galera operations require cluster quorum. Check `GaleraClusterSize` and `GaleraClusterStatus` before making changes.
- **Backup consistency**: Logical backups use `mysqldump` with `--single-transaction`. Physical backups use `mariabackup` with binlog position tracking.
- **Binary log retention**: PITR depends on binary logs. Ensure binlog retention is configured appropriately.
- **Password handling**: Never log passwords. Use `SecretKeyRef` patterns to reference passwords stored in Kubernetes Secrets.

## Code conventions

### Error handling

- Use `github.com/hashicorp/go-multierror` for error aggregation
- Wrap errors with `fmt.Errorf("context: %w", err)` for chain inspection
- Use `apierrors.IsNotFound(err)` for Kubernetes not-found checks
- Return `client.IgnoreNotFound(err)` in reconcile loops for cleanup operations
- Never swallow errors silently

### Logging

- Use `log.FromContext(ctx)` from controller-runtime
- Named loggers: `log.FromContext(ctx).WithName("component")`
- Verbose levels: `logger.V(1).Info("message")` for debug-level logging
- Never log secrets, passwords, or tokens

### Import organization

1. Standard library
2. Third-party imports
3. Project imports (`github.com/mariadb-operator/mariadb-operator/v26/...`)
4. Kubernetes imports (`k8s.io/...`, `sigs.k8s.io/...`)

### Builder pattern

Use `pkg/builder.Builder` to construct Kubernetes resources. The builder holds the scheme, environment, and discovery client. Methods like `BuildMariadbStatefulSet`, `BuildService`, `BuildBackupJob` etc. produce resources with proper labels, owner references, and environment configuration.

### Environment configuration

The operator reads configuration from environment variables via `pkg/environment.OperatorEnv`. Related images (MariaDB, MaxScale, Exporter) are injected at deployment time. Never hardcode image references.

## Testing

### Unit tests

- Use Ginkgo v2 / Gomega
- Structured in `*_test.go` files alongside source
- Run with `make test`

### Integration tests

- Use controller-runtime's `envtest` for an in-process fake Kubernetes API
- Test suite in `internal/controller/suite_test.go` installs cert-manager and volume snapshot CRDs
- Run with `make test-int`
- Tests are labeled (e.g. `Label("basic")`) for selective execution with `--label-filter=basic`

### E2E tests

- Use Ginkgo/Gomega with kind clusters
- Live against real Kubernetes clusters with real MariaDB pods
- Located in `test/e2e/`

## Codegen and generated files

- `make manifests` -- Generates CRDs, RBAC, webhook configs via controller-gen
- `make code` -- Generates DeepCopy methods
- `make embed-entrypoint` -- Fetches MariaDB docker entrypoint script
- `make helm-gen` -- Generates Helm chart from config/
- `make gen` -- Runs all code generation

**Never hand-edit** generated files in `config/crd/bases/`, `config/rbac/`, or deepcopy output.

## Gotchas and non-obvious rules

- **Module path is versioned**: The Go module path is `github.com/mariadb-operator/mariadb-operator/v26`. All internal imports use the `v26` prefix. When adding new imports, use the correct versioned path.
- **Phase ordering matters**: The reconciliation phase order in `mariadb_controller.go` is carefully designed. Infrastructure phases must complete before workload phases, which must complete before data phases. Adding a new phase requires understanding its dependencies.
- **envtest setup**: Integration tests require `controller-runtime/pkg/envtest` Kubernetes binaries. Run `make envtest` to install them. The `KUBEBUILDER_ASSETS` env var must point to the installed binaries.
- **CRD size limit**: CRDs have a hard Kubernetes limit of 1MB. The `make crd-size` target enforces a 900KB soft limit. Adding large OpenAPI v3 schemas to CRDs can exceed this limit.
- **Multi-command binary**: The `cmd/controller/main.go` uses cobra to support multiple commands (controller, webhook, backup CLI, restore CLI, pitr CLI, init, agent). The controller and webhook commands share the same binary.
- **Related images**: The operator does not bundle MariaDB, MaxScale, or exporter images. They are referenced via environment variables (`RELATED_IMAGE_*`) and must be available in the target cluster's registry.
- **Webhook certificates**: For local development, use `make cert-webhook` to generate self-signed certificates. The webhook command reads certs from `WEBHOOK_PKI_DIR`.
- **Galera library path**: The Galera shared library path (`MARIADB_GALERA_LIB_PATH`) differs between MariaDB image variants (alpine vs ubi). Set correctly in the Makefile or environment.
