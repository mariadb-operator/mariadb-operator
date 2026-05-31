**`{{ .ProjectName }}` [26.06](https://github.com/mariadb-operator/mariadb-operator/releases/tag/26.6.0) is here!** 🦭

Welcome to another release of `{{ .ProjectName }}`! This is a big one — we are introducing the **multi-cluster topology**, a game-changing feature that enables you to replicate data across multiple Kubernetes clusters for high availability, disaster recovery, and zero-downtime blue-green deployments.

We've also added a powerful **maintenance mode** for safe operational windows, **root password rotation** for seamless credential management, and a new way to consume our Helm charts via **OCI registries**.

Additionally, we have received a bunch of contributions from our amazing community during this release, including bug fixes and improvements. We feel very grateful for your efforts and support, thank you! 🙇‍♂️ Refer to the PRs in the changelog below for further details.

If you're upgrading from previous versions, __do not miss the [UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_26.6.0.md)__ for a smooth transition.

## Multi-cluster topology

The multi-cluster feature enables high availability by replicating data between multiple MariaDB clusters. It builds on top of either [replication](./replication.md) or [Galera](./galera.md) clusters, creating a topology where one cluster acts as the primary and the others as replicas, with each cluster maintaining its own internal HA mechanism.

A multi-cluster setup can be deployed in two ways:

**Across multiple Kubernetes clusters** — each Kubernetes cluster runs a MariaDB cluster with its own HA mechanism. The clusters are connected via remote replication, forming a hierarchy where the primary cluster receives all write operations and the replica clusters replicate data from it. This provides both intra-cluster HA (within each cluster) and inter-cluster HA (across Kubernetes clusters), making it ideal for multi-region deployments and disaster recovery.

**Within a single Kubernetes cluster** — a single Kubernetes cluster can host multiple MariaDB clusters with local replication configured between them. This is useful for blue-green deployments, where one cluster serves traffic while the other is updated in the background, enabling zero-downtime upgrades without data loss.

The operator handles the full lifecycle of this topology, including: provisioning the primary and replica MariaDB clusters, taking physical backups of the primary cluster, bootstrapping the replica cluster from the backup, configuring the replication connection between clusters, and performing cluster-level switchover when needed.

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
spec:
  multiCluster:
    enabled: true
    primary: mariadb-primary
    members:
      - name: mariadb-primary
        externalMariaDbRef:
          name: mariadb-primary
      - name: mariadb-replica
        externalMariaDbRef:
          name: mariadb-replica
```

Refer to the [multi-cluster docs](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/multi-cluster.md) for a complete guide and examples.

## Maintenance mode

The operator now provides a **maintenance mode** that allows you to safely perform maintenance operations on a MariaDB cluster. When enabled, maintenance mode gives you fine-grained control over how the database behaves during maintenance windows, including blocking new connections, draining existing connections, and setting the database to read-only mode.

This is particularly useful for cluster switchover in multi-cluster setups (preventing writes to the primary before switching to a replica), debugging by isolating the database from application traffic, or any operational task that requires controlled access.

The maintenance mode supports three composable modes:
- **Cordon mode**: blocks all new connections by removing Pods from service endpoints
- **Drain connections**: gracefully terminates long-running connections after a configurable grace period
- **Read-only mode**: sets the database to read-only, preventing any write operations while allowing reads

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
spec:
  maintenance:
    enabled: true
    cordon: true
    drainConnections: true
    drainGracePeriodSeconds: 30
    readOnly: true
```

MaxScale also supports maintenance mode via cordon functionality.

Refer to the [maintenance docs](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/maintenance.md) for a complete guide.

## Root password rotation

You can now rotate the root password of a `MariaDB` resource by simply updating the referenced `Secret`. The operator automatically handles the rotation process: it connects using the old password, issues `ALTER USER` commands to update the password, propagates the new password to all components (agent sidecars, Galera SST credentials), and even rolls back if the update fails to ensure consistency.

This enables seamless credential rotation without downtime, and works well with GitOps tools like sealed-secrets and external-secrets for managing secrets declaratively.

Refer to the [security docs](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/security.md) for details.

## Helm OCI charts

We are excited to announce that our Helm charts are now available via **OCI registries** on **GitHub Container Registry (GHCR)**. This is the recommended way to install and manage the MariaDB operator going forward.

OCI-based Helm charts offer a simpler experience — no need to add separate Helm repositories, manage credentials, or deal with registry-specific tooling. You can install directly from GHCR:

```bash
helm upgrade --install mariadb-operator oci://ghcr.io/mariadb-operator/charts/mariadb-operator --version 26.6.0
```

### Deprecation notice

> [!CAUTION]
> The `helm.mariadb.com` Helm repository is **deprecated** and will be removed in a future release. We strongly encourage migrating from the Helm registry approach to Helm OCI. Refer to the [upgrade guide](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_26.6.0.md) and [Helm docs](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/helm.md) for migration steps.
>
> Similarly, the `docker-registry*.mariadb.com` Docker registries are **deprecated**. Please migrate to the new registries. Refer to the [Docker docs](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/docker.md) for details.

## Bugfixes

- Fixed a `PhysicalBackup` deadlock that could occur during backup operations ([#1712](https://github.com/mariadb-operator/mariadb-operator/pull/1712))
- Fixed a stale staging area issue that could cause incorrect backup state ([#1744](https://github.com/mariadb-operator/mariadb-operator/pull/1744))
- Fixed MaxScale Pod initialization to properly account for services and listeners ([#1678](https://github.com/mariadb-operator/mariadb-operator/pull/1678))

## Improvements

- Added a restart trigger for the operator on ConfigMap changes ([#1667](https://github.com/mariadb-operator/mariadb-operator/pull/1667))
- Added replication role labels to Prometheus metrics for better observability ([#1716](https://github.com/mariadb-operator/mariadb-operator/pull/1716))
- Added `wait-for-it` support to the `mariadb-cluster` Helm chart ([#1739](https://github.com/mariadb-operator/mariadb-operator/pull/1739))

## Field support

- Added support for `TerminationGracePeriodSeconds` on MariaDB pods ([#1686](https://github.com/mariadb-operator/mariadb-operator/pull/1686))
- Added support for `Lifecycle` hooks on MariaDB pods ([#1687](https://github.com/mariadb-operator/mariadb-operator/pull/1687))
- Added `strategy` and `revisionHistoryLimit` configuration to Deployments in the Helm chart ([#1726](https://github.com/mariadb-operator/mariadb-operator/pull/1726))

---

## Community

Contributions of any kind are always welcome: adding yourself to the [list of adopters](https://github.com/mariadb-operator/mariadb-operator/blob/main/ADOPTERS.md), reporting issues, submitting pull requests, or simply starring the project! 🌟

## Enterprise

For enterprise users, see the __[MariaDB Enterprise Operator](https://mariadb.com/products/enterprise/kubernetes-operator/)__, a commercially supported Kubernetes operator from MariaDB with additional enterprise-grade features.
