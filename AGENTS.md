# AGENTS.md

Guidance for AI coding agents working in `mariadb-operator`. Read this before making changes.

## Project Overview

`mariadb-operator` is a Kubernetes operator that manages the full lifecycle of MariaDB: provisioning, high availability (MariaDB native replication and Galera), multi-cluster topologies, backups (logical and physical), point-in-time recovery, users/grants/databases as code, MaxScale proxying, TLS, metrics and updates.

- **Module**: `github.com/mariadb-operator/mariadb-operator/v26`.
- **API group**: `k8s.mariadb.com/v1alpha1`
- **Stack**: Go, controller-runtime, Kubebuilder markers + controller-gen, Ginkgo v2/Gomega, envtest, KIND, Helm
- **Docs**: `docs/*.md` (feature-oriented, one file per topic — see Feature Map)

## Simplicity

This project is **highly biased towards simplicity and against over-engineering** — every line merged is a line maintained:

- Prefer the simplest implementation that solves the actual problem; reuse existing patterns before introducing new abstractions, layers or dependencies.
- Complexity without a clear outcome is a red flag — speculative flexibility and unrealistic corner cases included.
- New config options, spec fields, flags and dependencies multiply the test matrix and maintenance burden — justify them with a concrete use case.

In scope decisions and reviews, treat unjustified complexity as a defect.

### CRDs

| Kind | Purpose |
|------|---------|
| `MariaDB` | Main resource: MariaDB servers with optional replication/Galera HA, multi-cluster, maintenance, TLS, metrics, storage |
| `MaxScale` | MaxScale database proxy connected to a MariaDB |
| `Backup` | Logical backups (mariadb-dump) with S3/Azure/PVC storage, scheduling and retention |
| `PhysicalBackup` | Physical backups via mariadb-backup CLI or Kubernetes `VolumeSnapshot` |
| `Restore` | Restores a MariaDB from a backup (`bootstrapFrom` also embeds this logic) |
| `PointInTimeRecovery` | PITR: continuous binlog archival (data-plane agent) + recovery to a target time. Short name: `pitr` |
| `Database`, `User`, `Grant` | SQL resources reconciled against a MariaDB/ExternalMariaDB |
| `SqlJob` | Runs arbitrary SQL scripts as Jobs/CronJobs |
| `Connection` | Generates connection string Secrets, with health checks |
| `ExternalMariaDB` | Represents a MariaDB outside the cluster; enables SQL resources against it and multi-cluster membership |

## Repository Structure

```
mariadb-operator/
├── .agents/skills/          # Agent skills (SKILL.md per dir); .claude/skills is a symlink to it so both
│                            #   OpenCode and Claude Code auto-discover them
├── api/v1alpha1/            # CRD types, kubebuilder markers, defaults, indexes (*_indexes.go), helpers
├── cmd/                     # Entrypoints (single binary, cobra subcommands)
│   ├── controller/          # Root cmd = operator; subcommands: webhook, cert-controller
│   ├── agent/               # Data-plane sidecar agent (replication, galera subcommands)
│   ├── init/                # Init container (replication, galera subcommands)
│   ├── backup/              # Backup CLI (nested `restore` subcommand)
│   └── pitr/                # PITR CLI (binlog archive/replay)
├── internal/
│   ├── controller/          # Reconcilers + integration tests (envtest, suite_test.go)
│   ├── webhook/v1alpha1/    # Validation webhooks (one per CRD)
│   └── helmtest/            # Helm chart unit tests (terratest)
├── pkg/                     # Reusable libraries
│   ├── builder/             # Builds all child K8s objects (StatefulSets, Jobs, Services...)
│   ├── controller/          # Sub-reconcilers: auth, batch, certificate, configmap, deployment,
│   │                        #   endpoints, galera, maintenance, pvc, rbac, replication, secret,
│   │                        #   service, servicemonitor, sql, statefulset
│   ├── sql/                 # MariaDB SQL client (users, grants, replication, Galera, binlogs, locks)
│   ├── condition/           # Status condition helpers (Ready / Complete)
│   ├── refresolver/         # Resolves refs to Secrets, ConfigMaps, MariaDB, MaxScale...
│   ├── watch/               # WatcherIndexer for indexed watches
│   ├── discovery/           # Runtime API discovery (cert-manager, ServiceMonitor, VolumeSnapshot, SCC)
│   ├── environment/         # Operator/pod env config (RELATED_IMAGE_*, MARIADB_OPERATOR_*)
│   ├── replication/, galera/# HA orchestration logic
│   ├── webhook/             # Immutability validation via struct tags
│   ├── embed/               # Embedded mariadb-docker entrypoint (generated — do not edit)
│   └── ...                  # backup, binlog, pki, health, metadata, pod, wait, etc.
├── config/                  # Kustomize sources (crd/, rbac/, webhook/, manager/, default/, samples/)
├── deploy/                  # Generated distributables
│   ├── charts/              # Helm charts: mariadb-operator, mariadb-operator-crds, mariadb-cluster
│   ├── crds/                # Rendered crds.yaml
│   └── manifests/           # Rendered install bundles
├── docs/                    # Feature docs + generated api_reference.md
├── examples/                # Example manifests (incl. multi-cluster/) and Flux GitOps setup
├── hack/                    # Dev/CI scripts (install_*.sh, config/, manifests/)
├── make/                    # Modular Makefiles: build, deploy, deps, dev, docs, gen, helm, net, pki, azure
├── test/e2e/                # E2E tests (Ginkgo) + test/utils/ — do NOT add new E2E tests for now
└── csi-driver-host-path/    # Vendored submodule — ignore
```

**Ignore `.gitignore`d paths.** Anything listed in `.gitignore` (vendored, generated or transient dirs) is **not** part of this repository — do not index, read for context, or modify it; changes there are never committed.

## Makefile Targets

The root `Makefile` includes `make/*.mk`. Run `make help` (or bare `make`) to list all targets. Most relevant:

| Target | Description |
|--------|-------------|
| `make build` | Build `bin/mariadb-operator` binary |
| `make docker-dev` | Build image and load it into the KIND cluster |
| `make run` | Run operator locally against current cluster (runs `lint` first) |
| `make webhook` | Run webhook server locally (generates dev certs via `cert-webhook`) |
| `make test` | Unit tests: `./api/...`, `./pkg/...`, `./internal/helmtest/...`, `./internal/webhook/...` |
| `make test-int` | Integration tests (`./internal/controller/...`, excludes `multi-cluster` label) |
| `make test-int-basic` | Integration tests labeled `basic` (what PR CI runs) |
| `make test-int-multi-cluster` | Multi-cluster labeled integration tests |
| `make lint` | golangci-lint |
| `make gen` | **All codegen** — manifests, deepcopy, embedded entrypoint, helm, bundles, docs, examples |
| `make manifests` | CRDs, RBAC, webhook configs via controller-gen |
| `make code` | DeepCopy methods via controller-gen |
| `make cluster` / `cluster-delete` | Create/delete KIND cluster |
| `make net` | Local networking: MetalLB + `/etc/hosts` entries (host ↔ cluster connectivity) |
| `make install` | CRDs + config + Prometheus CRDs + ServiceAccount + StorageClass + `docker-dev` |
| `make install-minio` / `install-cert-manager` / `install-prometheus` / `install-azurite` / `install-csi-hostpath` | Optional per-feature dependencies |
| `make crd-size` | Fail if generated CRDs exceed 900KB (K8s hard limit is 1MB) |
| `make dump` | Dump cluster state (CRs, pods, events, logs) for debugging |
| `make release` | Test release locally with goreleaser |

Pass extra Ginkgo args with `TEST_ARGS`, e.g. `make test-int TEST_ARGS="--focus='some spec'"` or `TEST_ARGS="--label-filter=basic"`.

## Development Guides

### Environment setup

```bash
make cluster        # KIND cluster
make install        # CRDs + dev dependencies + operator image
make net            # MetalLB + /etc/hosts so host-run operator can reach MariaDB pods
make run            # operator on your host, against the KIND cluster
```

Feature-specific extras before testing those areas: `make install-minio` (S3 backups), `make install-azurite` (Azure backups), `make install-cert-manager` (TLS via cert-manager), `make install-prometheus` (metrics), `make install-csi-hostpath` (VolumeSnapshots).

### Development workflows

- **Change API types** (`api/v1alpha1/`) → `make gen`, then commit the regenerated files (CI fails otherwise).
- **Change code** → `make build` compiles; `make run` to exercise against KIND.
- **Lint** → `make lint`. Enabled linters (`.golangci.yml`, v2 config): bodyclose, errcheck, ginkgolinter, gocyclo (≤22), govet, ineffassign, lll (≤140), misspell, nestif (≤12), noctx, staticcheck, unused. Formatters: gofmt + goimports.
- **Test** → `make test` for unit; `make test-int-basic` for a fast integration signal. Integration tests need a KIND cluster with `make install`, `make install-minio`, `make net` (and `make install-azurite` for Azure specs).

### Adding a new feature

1. **API**: add fields/types in `api/v1alpha1/`. New fields must be optional (`omitempty`, `+optional`) with kubebuilder validation markers. Prefer CEL (`+kubebuilder:validation:XValidation`) for static rules; document constraints in the field comment.
2. **Generate**: `make gen` — regenerates deepcopy, CRDs, helm chart CRDs, bundles and API docs.
3. **Webhook**: extend the CRD's validator in `internal/webhook/v1alpha1/`. Mark immutable fields with the immutability struct tags (see below).
4. **Controller**: implement reconciliation. For MariaDB features, add a phase to the ordered `phases` slice in `internal/controller/mariadb_controller.go` (position matters — see Code Patterns) and/or a sub-reconciler under `pkg/controller/<feature>/`.
5. **Builder**: child Kubernetes objects are constructed in `pkg/builder/` so labels, metadata and owner references stay consistent.
6. **RBAC**: add `//+kubebuilder:rbac:` markers next to the reconciler and rerun `make manifests`. Then **manually promote** the new rules from `config/rbac/role.yaml` into the Helm chart's templated RBAC (see Gotchas).
7. **Tests**: unit tests alongside code; integration tests in `internal/controller/*_test.go` — label cheap smoke specs with `Label("basic")` so they run in PR CI.
8. **Docs + example**: update the relevant `docs/<feature>.md` and add an example under `examples/manifests/` when user-facing.

### Codegen and generated files

`make gen` is the single command; CI's **Artifacts** job runs it and fails the PR if the diff is not committed. Note: when `VERSION` contains `-dev`, `gen` runs a reduced set (no docs/bundles/examples).

**Never hand-edit** (regenerate instead):

- `api/v1alpha1/zz_generated.deepcopy.go` → `make code`
- `config/crd/bases/`, `config/rbac/role.yaml`, webhook configs → `make manifests`
- `deploy/crds/`, `deploy/manifests/`, `deploy/charts/mariadb-operator-crds/templates/crds.yaml` → `make manifests-bundle` / `make helm-crds`
- Helm chart `README.md` files → `make helm-docs` (edit `README.md.gotmpl` / `values.yaml` comments instead)
- `docs/api_reference.md` → `make docs-api` (edit Go type comments instead)
- `docs/docker.md` → `make docs-docker` (edit `docs/docker.md.gotmpl`)
- `pkg/embed/mariadb-docker/` → `make embed-entrypoint` (pinned to a mariadb-docker commit in the Makefile)
- `examples/manifests/` image versions → `make examples`

## Architecture and Code Patterns

### Phase-based reconciliation

The MariaDB controller iterates an **ordered** slice of `reconcilePhaseMariaDB` in `internal/controller/mariadb_controller.go`. Current order:

```
Spec → Status → Suspend → Secret → ConfigMap → TLS → RBAC → Init → Scale out →
Replica recovery → Storage → StatefulSet → PodDisruptionBudget → Service →
Replication → Labels → Galera → Root Password → Restore → PITR → MultiCluster →
Maintenance → SQL → Metrics → Connection
```

Each phase returns `(ctrl.Result, error)`; the loop short-circuits on a non-zero `Result`, patches status on errors, and supports skipping via `ErrSkipReconciliationPhase`. **Ordering matters**: infrastructure (Secrets, ConfigMaps, TLS, RBAC) before workloads (StatefulSet, Services), before data/topology operations (Replication, Galera, Restore, SQL). Insert new phases at the point where their dependencies are already reconciled.

### Sub-reconcilers

Child-resource reconciliation lives in `pkg/controller/<kind>/` (statefulset, service, secret, configmap, rbac, certificate, galera, replication, maintenance, batch, sql, ...). They are injected into controllers, build desired state via `pkg/builder`, and create-or-patch idempotently.

### Generic SQL reconciler

`pkg/controller/sql/` provides a generic reconciler shared by User, Grant and Database controllers. Interfaces live in `pkg/controller/sql/types.go`: `Resource` (the CRD), `WrappedReconciler` (resource-specific SQL + status patching), `Finalizer` (cleanup on deletion). It handles MariaDB ref resolution, health waiting, SQL client lifecycle and requeue with jitter — implement the interfaces rather than re-writing this flow.

### Status patching

Status is always patched separately from spec — never `Update` the whole object — with `client.MergeFrom` and a patcher function:

```go
patch := client.MergeFrom(mariadb.DeepCopy())
patcher(&mariadb.Status)
return r.Status().Patch(ctx, mariadb, patch)
```

Each controller exposes a `patchStatus(ctx, obj, patcher)` helper following this shape (e.g. `internal/controller/mariadb_controller.go`); reuse it rather than patching inline. The phase loop also patches status automatically when a phase errors.

### Conditions

Conditions live in `pkg/condition` with the types defined in `api/v1alpha1/condition_types.go`: `Ready` for long-lived resources (MariaDB, MaxScale, SQL resources) and `Complete` for one-shot resources (Backup, Restore, SqlJob). Use the existing helpers (`SetReadyHealthy`, `SetReadyWithStatefulSet`, `SetReadyUnhealthyWithError`, `SetInitialized`, `SetUpdated`, ...) instead of setting conditions manually — they keep reasons and messages consistent across CRDs, and status consumers (e.g. `kubectl wait --for=condition=ready`) depend on them.

### Builder

All child Kubernetes objects come from `pkg/builder.Builder` (e.g. `BuildMariadbStatefulSet`, `BuildService`, `BuildBackupJob`, `BuildPITRJob`, `BuildVolumeSnapshot`). The builder applies consistent labels, metadata and owner references (`ctrl.SetControllerReference`) so children are garbage-collected with their parent.

### References, watches and discovery

- **`pkg/refresolver`** resolves cross-resource references (Secrets, ConfigMaps, MariaDB, MaxScale, ExternalMariaDB).
- **`pkg/watch.WatcherIndexer`** + `Index*` functions in `api/v1alpha1/*_indexes.go` set up indexed watches so changes to referenced objects (e.g. a password Secret) re-trigger reconciliation of dependents.
- **`pkg/discovery`** detects optional APIs at runtime (cert-manager `Certificate`, `ServiceMonitor`, `VolumeSnapshot`, OpenShift SCC). Gate optional-dependency logic behind these checks — never assume the CRDs exist.

### Webhooks and immutability

Validation-only admission webhooks (no defaulting webhooks — defaults are set in `setSpecDefaults`/API helpers) live in `internal/webhook/v1alpha1/`, one validator per CRD implementing a generic `admission.Validator[*T]`. Immutable fields are enforced by `pkg/webhook/inmutable_webhook.go` through struct tags. Call the immutability validator first in `ValidateUpdate`. Simple static rules increasingly use CEL markers directly on types.


### Kubernetes best practices

- **Owner references**: always set on child resources for GC; `pkg/builder` does this (see Builder) — another reason to build children there.
- **Finalizers**: use them for ordered cleanup on deletion (the generic SQL reconciler already implements this flow). Use `client.IgnoreNotFound()` when deleting resources in finalizers.
- **Reconcile idempotency**: every reconcile must converge without side effects on re-run. Read first (`Get`), then `Create`/`Patch` — never assume state.
- **Status subresource**: patch status separately from spec, never mutate spec during status updates (see Status patching).
- **Requeue with backoff**: return `ctrl.Result{RequeueAfter: ...}` for expected wait conditions. The SQL reconciler adds a random offset (`RequeueMaxOffset`) to spread requeues and avoid thundering herds — preserve that.
- **Concurrency limits**: controllers set `controller.Options{MaxConcurrentReconciles: n}`; the heavy ones (MariaDB, MaxScale, PhysicalBackup) expose per-controller `--*-max-concurrent-reconciles` flags (default 10) in `cmd/controller/main.go`.
- **Leader election**: only one instance reconciles at a time (single replica by default; enabling HA in the chart, `ha.enabled`, runs multiple replicas with `--leader-elect`, which guarantees a single leader), so don't guard against separate instances racing — but a controller still reconciles objects in parallel (`MaxConcurrentReconciles`), so keep reconcile logic idempotent and free of cross-object shared state.
- **RBAC markers**: declare permissions with `//+kubebuilder:rbac:` markers next to the reconciler that needs them, then `make manifests`. Remember the manual promotion into the Helm chart (see Gotchas).

### Error handling

- Wrap errors with `fmt.Errorf("...: %w", err)` so callers can inspect the chain; never swallow errors silently.
- Aggregate multiple errors with `github.com/hashicorp/go-multierror` (e.g. reconcile error + status patch error).
- Use `apierrors.IsNotFound(err)` for Kubernetes not-found checks, and `client.IgnoreNotFound(err)` for deletes/cleanup where absence is fine.
- In phase-based reconciliation, returning an error patches status with the failure; return `ErrSkipReconciliationPhase` to skip a phase intentionally instead of faking success.

### Logging

- Get loggers from context: `log.FromContext(ctx).WithName("component")` — never construct ad-hoc loggers.
- Reserve `Info` for state changes an operator of the system cares about; use `logger.V(1).Info(...)` for debug-level detail.
- Include CR identity (the logger from the reconcile context already carries name/namespace) rather than re-formatting it into the message.
- **Never log passwords, tokens, certificates or SQL statements containing credentials.**

### Events

Surface user-visible state transitions as Kubernetes Events (visible in `kubectl describe` / `kubectl get events`), in addition to conditions and logs — they are the primary way operators observe what the reconciler did.

- Controllers hold an `events.EventRecorder` (`k8s.io/client-go/tools/events`, not the legacy `record.EventRecorder`) in a `Recorder` field, obtained from the manager via `mgr.GetEventRecorder("<name>")` and wired per subsystem in `cmd/controller/main.go`. Sub-reconcilers (`pkg/controller/galera`, `replication`, `certificate`, `maintenance`) get one via constructor injection.
- **Reason and action strings are centralized**: reason constants (`Reason*`) live in `api/v1alpha1/event_types.go` and action constants (`Action*`, e.g. `ActionReconciling`) in `api/v1alpha1/event_actions.go` — reuse an existing constant or add one there; never inline a string literal.
- Emit with `Eventf(regarding, related, eventtype, reason, action, note, args...)`: pass a `Reason*` constant for `reason`, an `Action*` constant for `action`, and `corev1.EventTypeNormal` / `corev1.EventTypeWarning` for the type. Never put secrets in the note (same rule as Logging).

## Feature Map

Where to look when working on a specific feature:

| Feature | Docs | Key code |
|---------|------|----------|
| Replication | `docs/replication.md` | `pkg/replication/`, `pkg/controller/replication/`, `api/v1alpha1/mariadb_replication_types.go` |
| Galera | `docs/galera.md` | `pkg/galera/`, `pkg/controller/galera/`, `cmd/agent`, `cmd/init` |
| Multi-cluster topology | `docs/multi-cluster.md` | `api/v1alpha1/mariadb_multi_cluster_types.go`, `internal/controller/mariadb_controller_multicluster*.go`, `examples/manifests/multi-cluster/` |
| ExternalMariaDB | `docs/external_mariadb.md` | `api/v1alpha1/external_mariadb_types.go` |
| Maintenance mode | `docs/maintenance.md` | `MariaDBMaintenance` in `api/v1alpha1/mariadb_types.go`, `pkg/controller/maintenance/` — cordon / drainConnections / readOnly; keeps reconciling (unlike suspend) |
| Suspend | `docs/suspend.md` | `SuspendTemplate` in specs; "Suspend" phase halts the whole reconcile loop |
| Root password rotation | — | `internal/controller/mariadb_controller_root_password.go` — reconciles root password from the Secret contents (the ref itself is immutable-init), propagates to data-plane and `wsrep_sst_auth`, tracks `status.rootPasswordHash` |
| Logical backup | `docs/logical_backup.md` | `internal/controller/backup_controller.go`, `cmd/backup`, `pkg/backup/` |
| Physical backup / VolumeSnapshot | `docs/physical_backup.md` | `internal/controller/physicalbackup_controller*.go`, `pkg/volumesnapshot/` |
| PITR | `docs/pitr.md` | `api/v1alpha1/pointintimerecovery_types.go`, `internal/controller/mariadb_controller_pitr.go`, `cmd/pitr`, `pkg/binlog/` |
| Data-plane (agent/init) | `docs/data_plane.md` | `cmd/agent`, `cmd/init`, `pkg/agent/` |
| MaxScale | `docs/maxscale.md` | `internal/controller/maxscale_controller*.go`, `pkg/maxscale/` |
| TLS | `docs/tls.md` | `pkg/pki/`, `pkg/controller/certificate/`, `cmd/controller/cert_controller.go` |
| Updates | `docs/updates.md` | `updateStrategy`: `ReplicasFirstPrimaryLast` (default), `RollingUpdate`, `OnDelete`, `Never` |
| Helm charts | `docs/helm.md` | `deploy/charts/` — `mariadb-operator`, `mariadb-operator-crds`, `mariadb-cluster`; published to GitHub Pages and OCI (`oci://ghcr.io/mariadb-operator/charts/<chart>`) via `.github/workflows/helm*.yml` |

## Testing

- **Unit** (`make test`): Ginkgo/Gomega over `api/`, `pkg/`, `internal/helmtest/` (chart rendering via terratest), `internal/webhook/` (envtest).
- **Integration** (`make test-int`): `internal/controller/*_test.go` against envtest, bootstrapped by `suite_test.go`. Ginkgo labels tier the suite: `basic` (PR smoke set), `multi-cluster` (separate target), `flaky`, `finalizer`. Requires a KIND cluster prepared with `make install`, `make install-minio`, `make net` (+ `make install-azurite` for Azure specs).

### Local cluster connectivity

Whether you `make run` the operator or `make test-int`, the binary runs **on your host** while the KIND cluster (and its MariaDB pods) runs inside a container — and the SQL client (`pkg/sql`) connects to the pods over their internal Kubernetes FQDNs.

Because of that host-to-container split, connectivity to the pods has to be simulated. That is what `make net` (`install-metallb` + `host`) provides:

- **MetalLB** assigns each MariaDB test Service a `type: LoadBalancer` IP pinned via a `metallb.io/loadBalancerIPs` annotation, using an IP derived from the KIND Docker network CIDR (`pkg/docker`, `hack/get_kind_cidr_prefix`). Those IPs are routable from the host because KIND's Docker bridge is directly reachable.
- **`/etc/hosts`** entries (added by `make net` → `hack/add_host.sh`) map the in-cluster FQDNs to those MetalLB IPs, so the host-side `sql.Open` resolves them.

Implications when writing integration tests: `make net` is a hard prerequisite (connections fail without it); a new MariaDB in a spec needs a unique pinned LB IP.

## CI — what a PR must pass

`.github/workflows/ci.yml` (docs/examples/markdown-only changes are skipped by a noop detector):

1. **Lint** — golangci-lint
2. **Typos** — `crate-ci/typos` over `api/ cmd/ internal/ pkg/`
3. **Build** — `make build` + `make docker-build`
4. **Unit tests** — `make helm-crds` + `make test`
5. **Integration tests** — `make test-int-basic` on regular PRs; full `test-int` + `test-int-multi-cluster` on `main` and `release*`/`feature*`/`fix*` branches
6. **Artifacts** — runs `make gen` and **fails if any generated file differs from what is committed**, plus `make crd-size` (900KB limit)

`.github/workflows/helm.yml` additionally lints/installs the three charts with chart-testing. Before pushing: `make gen && make lint && make test` at minimum.

## Gotchas and Non-obvious Rules

- **Phase order is load-bearing**: adding/moving phases in `mariadb_controller.go` changes bootstrap and recovery semantics. Understand a phase's dependencies before repositioning it.
- **Chart RBAC is NOT generated — promote it manually**: `//+kubebuilder:rbac:` markers only regenerate `config/rbac/role.yaml` (via `make manifests`/`make gen`). The Helm chart's RBAC under `deploy/charts/mariadb-operator/templates/` is templated (Helm conditionals like `currentNamespaceOnly`), so codegen cannot write it. When you add or change RBAC markers, manually replicate the new rules into the chart's templated RBAC files — `operator/rbac.yaml` (cluster-wide) **and** `operator/rbac-namespace.yaml` (namespace-scoped), plus the `cert-controller`/`webhook` variants if they are affected. The Artifacts CI job will not catch a missing promotion; a chart-deployed operator will fail at runtime with authorization errors.
- **CRD size budget**: the combined CRDs must stay under 900KB (`make crd-size`). Large inlined OpenAPI schemas (e.g. embedding full PodSpec-like structs) can blow the 1MB Kubernetes limit; the repo uses trimmed-down local copies of Kubernetes types in `api/v1alpha1/kubernetes_types.go` partly for this reason.
- **Single multi-command binary**: `cmd/controller` root command *is* the operator; `webhook` and `cert-controller` are subcommands, `backup` nests a `restore` subcommand, and `agent`/`init` have `replication`/`galera` subcommands.
- **Related images are external**: MariaDB, MaxScale and exporter images are not built here — they come from `RELATED_IMAGE_*` env vars set at deploy time (defaults in the root `Makefile`).
- **Suspend vs maintenance**: `spec.suspend` stops reconciliation entirely; `spec.maintenance` keeps reconciling while cordoning/draining/setting read-only. Don't conflate them.
- **Webhooks don't default**: defaulting happens in the controller's `Spec` phase (`setSpecDefaults`) and API helper methods, not in mutating webhooks (there are none).

## Safety Guardrails

**This operator manages production databases. Bugs can cause data loss, downtime or split-brain.**

### Secrets and credentials

- Never commit passwords, tokens, private keys or certificates. Credentials are always consumed via `SecretKeyRef`-style references; generated passwords come from `pkg/password`.
- Never write secret values into status, events, annotations or error messages (see Logging for what counts as a secret and the never-log rule).
- Test fixtures use obviously fake credentials (e.g. `MariaDB11!`); keep it that way.

### Backward compatibility

- Do not remove or rename existing spec/status fields, CRD kinds, or enum values. Deprecate instead, keeping old fields functional.
- New spec fields must be optional with safe defaults so existing CRs are unaffected on upgrade.
- Do not change defaults in ways that alter behavior of existing clusters, and do not tighten validation (webhook or CEL) so that previously-valid stored objects fail on update.
- Respect existing immutability contracts (the immutability struct tags): making a mutable field immutable, or vice versa, is a breaking change.
- Kubernetes object names, labels and selectors generated by `pkg/builder` are contracts — changing them orphans or recreates resources in running clusters (StatefulSet selectors are immutable).

### Database availability and data loss

- Be extremely careful in replica recovery, switchover and Galera recovery logic (`pkg/replication/`, `pkg/galera/`, `pkg/controller/galera/`): wrong sequencing risks split-brain or unrecoverable clusters. Quorum-affecting operations must check cluster state first.
- When changing primaries or writing to a primary, ensure replicas are in sync (`WaitForReplicaGtid`) before proceeding.
- Do not weaken backup/restore semantics: PITR depends on gapless binlog archival; restore flows coordinate multi-step sequences that must not be interrupted mid-way.
- Do not remove or reorder finalizer logic casually — it implements ordered cleanup of SQL resources, PVCs and snapshots (mechanism under Kubernetes best practices).

### Rolling restarts

- **Avoid touching the StatefulSet Pod template unless the feature requires it.** Any Pod template change triggers a rolling update of every MariaDB cluster managed by an upgraded operator (per `spec.updateStrategy`). Prefer changes that don't alter the template hash (e.g. lazily-read Secrets/ConfigMaps, agent API calls) over env/volume/container mutations.
- The same applies to Services and selector labels: churn there can drop client connections (cordon logic manipulates endpoint selectors deliberately — don't interfere with it accidentally).

### CI is the gate

- Every PR must pass all CI checks (see CI — what a PR must pass); run `make gen && make lint && make test` before pushing.
- Never merge with a red Artifacts job by hand-editing generated files — fix the source and regenerate.
