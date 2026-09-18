**`{{ .ProjectName }}` [26.10](https://github.com/mariadb-operator/mariadb-operator/releases/tag/26.10.0) is here!** 🦭

Before diving in, let's celebrate two milestones that the community has helped us reach: `{{ .ProjectName }}` just crossed [**1K stars on GitHub**](https://github.com/mariadb-operator/mariadb-operator) 🌟 and our Docker image reached [**11M pulls**](https://github.com/mariadb-operator/mariadb-operator/pkgs/container/mariadb-operator) 📦. This manifests the incredible adoption of the project, and we are thrilled to have you on board! If you haven't already, consider starring the repo and telling your peers about it!

This is a feature-packed release: we are introducing support for **MariaDB 12.3**✨, a series of important **replication topology** enhancements, several **Galera** improvements and **ZSTD compression** for backups.

A big part of this release comes from our amazing community: contributors are credited inline in the sections below. We feel very grateful for your efforts and support, thank you! 🙇‍♂️

If you're upgrading from previous versions, __do not miss the [UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_26.10.0.md)__ for a smooth transition.

## MariaDB 12.3 support

`{{ .ProjectName }}` now supports **MariaDB 12.3**, the latest [long-term support (LTS)](https://mariadb.org/mariadb-server-12-3-lts-released/) release of MariaDB, and uses it as the default version ([#1899](https://github.com/mariadb-operator/mariadb-operator/pull/1899)). The default `mariadb` image has been bumped to `mariadb:12.3.3`, so new clusters will run on it unless you pin a different image:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  image: mariadb:12.3.3
  # [...]
```

Existing clusters keep the image pinned in their spec and are not affected by this change.

## Replication topology

Our replication topology received a series of reliability and correctness improvements in this release:

- The operator no longer issues `RESET MASTER;` when configuring a replica ([#1883](https://github.com/mariadb-operator/mariadb-operator/pull/1883)). Purging a node's binary logs is not required to configure replication, and it is destructive once the node is promoted: its binary log history is gone. So far, we were only preserving it is for [point-in-time recovery](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/pitr.md) and [multi-cluster](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/multi-cluster.md), but we are now enforcing this for all topologies.
- Nodes that rejoin the cluster after a hard failover with self-owned GTIDs are now demoted using `MASTER_DEMOTE_TO_SLAVE` ([#1894](https://github.com/mariadb-operator/mariadb-operator/pull/1894)). This non-destructive replacement of `RESET MASTER` unblocks a scenario where a rejoining primary could never converge back to the cluster.
- The operator now enforces the correct `rpl_semi_sync_master_enabled` state on every node: enabled in the primary and disabled in the replicas ([#1902](https://github.com/mariadb-operator/mariadb-operator/pull/1902)). Previously every node was a semi-synchronous primary, including the replicas, so a replica could stall writes waiting for a semi-synchronous ACK that can never arrive. 
- A new optional field `replication.semiSyncBootAsReplica` (disabled by default) boots the node `read_only` with the semi-synchronous primary disabled, so a node is never writable while it is unable to require a replica acknowledgement ([#1902](https://github.com/mariadb-operator/mariadb-operator/pull/1902)).
- A new `replication.semiSyncWaitNoSlave` field exposes `rpl_semi_sync_master_wait_no_slave` ([#1885](https://github.com/mariadb-operator/mariadb-operator/pull/1885)). Disabling it makes a node with no semi-synchronous replicas connected fall back to asynchronous commit immediately, instead of stalling for the whole `semiSyncAckTimeout`.
- Switchover no longer leaves the new primary half-configured: when clearing `gtid_slave_pos`, an `Error 1948` was returned in certain circumstances. The operator handles now this situation and the primary is now fully configured, even when `Error 1948` is returned. ([#1886](https://github.com/mariadb-operator/mariadb-operator/pull/1886))
- `read_only` drift on replicas is now corrected even while the replica is unhealthy, as long as the cluster is not attached to MaxScale ([#1884](https://github.com/mariadb-operator/mariadb-operator/pull/1884)).

Refer to the [replication docs](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/replication.md) for a complete guide.

## Galera improvements

> [!IMPORTANT]
> The operator no longer recovers a Galera cluster where all members present an empty state (`00000000-0000-0000-0000-000000000000` UUID, `seqno: -1`) for safety reasons. In this situation, the user must choose where to bootstrap the new cluster via `forceClusterBootstrapInPod`, please refer to the [Galera documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/galera.md#force-cluster-bootstrap) for doing so.

- Logical backups of Galera clusters are now restorable ([#1836](https://github.com/mariadb-operator/mariadb-operator/pull/1836)). The dump excludes the Galera-managed `mysql.wsrep_*` system tables, whose `DROP TABLE` statements Galera denies on restore, and which it recreates on bootstrap anyway. Kudos to @ioanalytica!
- Recovery no longer bootstraps from a Pod with an empty state (`00000000-0000-0000-0000-000000000000` UUID, `seqno: -1`) just because it reports `safe_to_bootstrap: 1`, which could recreate an empty cluster and lose data ([#1816](https://github.com/mariadb-operator/mariadb-operator/pull/1816)). The bootstrap source is now selected by highest valid `seqno`, and a timed-out bootstrap keeps the recovered sequence numbers instead of discarding them. Kudos to @vixns!

## ZSTD compression

[Backup](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/logical_backup.md), [PhysicalBackup](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/physical_backup.md) and [PointInTimeRecovery](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/pitr.md) now support `zstd` as a compression algorithm, alongside `none`, `bzip2` and `gzip` ([#1874](https://github.com/mariadb-operator/mariadb-operator/pull/1874)). ZSTD offers an excellent compression/decompression speed with a good compression ratio. A new `compressionThreads` field lets you cap the number of CPU threads used during compression:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: mariadb-backup
spec:
  compression: zstd
  compressionThreads: 4
  # [...]
```

Kudos to @snaax for driving this feature end to end!

## Bugfixes

- Repeatable `mariadb-dump` flags such as `--ignore-table` are no longer silently dropped when passed multiple times in `spec.args`, and user-supplied `mariadb-backup` `--databases-exclude` values are now merged with the built-in ones instead of overriding them ([#1836](https://github.com/mariadb-operator/mariadb-operator/pull/1836), thanks @ioanalytica!)
- The replication liveness probe no longer fails (and the Pod no longer restarts) when the replica SQL thread is stopped administratively without error, e.g. during a physical backup with `--safe-slave-backup` ([#1882](https://github.com/mariadb-operator/mariadb-operator/pull/1882))
- `PhysicalBackup` jobs now inherit `podSecurityContext.seLinuxOptions` from the referenced `MariaDB` when the job does not set its own, fixing `Errcode: 13` failures on SELinux-enforced clusters ([#1889](https://github.com/mariadb-operator/mariadb-operator/pull/1889), thanks @henryjarend!)
- Scheduled `Backup` and `SqlJob` resources now report the `CronJobFailed` condition reason when the last run has failed, instead of `CronJobScheduled` ([#1867](https://github.com/mariadb-operator/mariadb-operator/pull/1867), thanks @SynSnt!)

## Improvements

- `mariadb-cluster` Helm chart now supports `MaxScale` resources ([#1909](https://github.com/mariadb-operator/mariadb-operator/pull/1909), thanks @hedgieinsocks!)
- Helm chart releases now include generated release notes, so tools like Renovate and Dependabot can show what changed between chart versions ([#1805](https://github.com/mariadb-operator/mariadb-operator/pull/1805), thanks @hedgieinsocks!)
- The Helm docs now describe scoping the operator to a specific set of namespaces via `WATCH_NAMESPACE` ([#1868](https://github.com/mariadb-operator/mariadb-operator/pull/1868), thanks @vshulcz!)
- Bumped the toolchain to Go 1.27 and golangci-lint v2.13.2 ([#1903](https://github.com/mariadb-operator/mariadb-operator/pull/1903))

---

## Community

Contributions of any kind are always welcome: adding yourself to the [list of adopters](https://github.com/mariadb-operator/mariadb-operator/blob/main/ADOPTERS.md), reporting issues, submitting pull requests, or simply starring the project! 🌟

## Enterprise

For enterprise users, see the __[MariaDB Enterprise Operator](https://mariadb.com/products/enterprise/kubernetes-operator/)__, a commercially supported Kubernetes operator from MariaDB with additional enterprise-grade features.
