# External replication

`mariadb-operator` supports replication from an external MariaDB instance i.e. running outside of the Kubernetes cluster where the operator runs. This feature allows us to create a cluster of replicas of an external MariaDB.

## Table of contents
<!-- toc -->
- [`ExternalMariaDB` configuration](#externalmariadb-configuration)
- [`MariaDB` configuration](#mariadb-configuration)
- [Bootstrapping and recovery sources](#bootstrapping-and-recovery-sources)
- [Replication initialization](#replication-initialization)
- [Scaling out](#scaling-out)
- [Replica recovery](#replica-recovery)
  - [Errors that trigger a recovery](#errors-that-trigger-a-recovery)
  - [How recovery works](#how-recovery-works)
  - [Timeouts](#timeouts)
- [Backup validity and retention](#backup-validity-and-retention)
- [Filtered replication](#filtered-replication)
  - [How the filtered backup is taken](#how-the-filtered-backup-is-taken)
- [Replicating through a MaxScale Binlogrouter](#replicating-through-a-maxscale-binlogrouter)
- [Services considerations](#services-considerations)
<!-- /toc -->

## `ExternalMariaDB` configuration

To setup the external replication first we need to add our source MariaDB as an `ExternalMariaDB`:
```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: external-mariadb
spec:
  host: mariadb.example.com
  port: 3306
  username: root
  passwordSecretKeyRef:
    name: mariadb
    key: password
  connection:
    secretName: external-mariadb
    healthCheck:
      interval: 5s
```

See [External MariaDB](./external_mariadb.md) for the full `ExternalMariaDB` reference, including TLS configuration.

## `MariaDB` configuration

With the `ExternalMariaDB` created, we just need to define a regular `MariaDB` object with replication enabled and use the `replicaFromExternal` property to point it to our external database:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: external-replicas
spec:
  storage:
    size: 10Gi
  replicas: 3
  replication:
    enabled: true
    replica:
      bootstrapFrom:
        physicalBackupTemplateRef:
          name: physicalbackup-external
    replicaFromExternal:
      mariaDbRef:
        name: external-mariadb
        kind: ExternalMariaDB
      serverIdOffset: 30
  service:
    type: ClusterIP
  primaryService:
    type: ClusterIP
  secondaryService:
    type: ClusterIP
```

When applied it will create 3 new replicas from the external database. The operator will create a logical backup of the external MariaDB, restore it on each Pod and configure the replication.

The `replicaFromExternal` field supports the following options:

| Field | Description | Default |
| ----- | ----------- | ------- |
| `mariaDbRef` | Reference to the `ExternalMariaDB` object that acts as the replication source. Immutable. | - |
| `serverIdOffset` | `serverId` offset value used on the replicas, to avoid conflicting with other replicas or with the source server. | `0` |
| `gtid` | Global Transaction ID position mode used when connecting a replica to the source (`CurrentPos` or `SlavePos`). | `CurrentPos` |
| `connectionTimeout` | Timeout used when the replica connects to the source. | - |
| `connectionRetries` | Number of connection retries when the replica connects to the source. | - |
| `healthCheckInterval` | Interval used to health-check the connection to the source. | `15s` |
| `filteredReplicaTables` | Optional list of `database.table` entries to replicate. See [Filtered replication](#filtered-replication). | - |

## Bootstrapping and recovery sources

New replicas (created during the initial provisioning, when scaling out, or during recovery) are bootstrapped from the data sources defined in `replica.bootstrapFrom`:

```yaml
spec:
  replication:
    enabled: true
    replica:
      bootstrapFrom:
        physicalBackupTemplateRef:
          name: physicalbackup-external
        logicalBackupTemplateRef:
          name: backup-external
    replicaFromExternal:
      mariaDbRef:
        name: external-mariadb
        kind: ExternalMariaDB
```

* `physicalBackupTemplateRef` (**required**): reference to a `PhysicalBackup` object used as a template to create a new `PhysicalBackup`. This is used to synchronize the data from an **up to date replica** to a new replica being bootstrapped during scale out and recovery. Its `target` should typically be set to `PreferReplica` so the backup is taken from a healthy replica when one is available (see [Replica recovery](#replica-recovery)).
* `logicalBackupTemplateRef` (optional): reference to a `Backup` object used as a template for the logical `Backup` taken **from the external MariaDB** during initialization and as a fallback during recovery. The template's `Spec` is copied over (resources, Pod template, etc.) and the operator overrides the fields it manages automatically (`mariaDbRef`, `storage`, `args`, `tables`, `compression`, `maxRetention`). If not provided, the operator uses sensible defaults for the logical backup.

Because `bootstrapFrom` is consumed by the scale out and recovery flows, and **replica recovery is enabled by default for external replication**, `bootstrapFrom` is effectively required: if it is not provided, scale out and recovery operations return an error. Recovery can be tuned (or disabled) through `replica.recovery`:

```yaml
spec:
  replication:
    replica:
      recovery:
        enabled: true
        errorDurationThreshold: 5m
```

When replicating from an external server, connection issues with the source are more likely. The following `replica` options are useful to avoid dropping read traffic in those situations:

* `ignoreMaxLagSeconds`: keep forwarding read queries to a replica even if it lags behind the source.
* `ignoreReplicationLivenessProbes`: keep forwarding read queries to a replica even if the replication liveness checks fail.

## Replication initialization

When a new external-replica `MariaDB` cluster is created, the operator runs a **replication initialization** before configuring replication. This happens once per cluster (tracked by the `ExternalReplInitialized` status condition):

1. **Take a logical backup from the external MariaDB.** The operator creates a logical `Backup` object pointing to the `ExternalMariaDB`. The dump is taken with `--master-data=1 --gtid --single-transaction` so it captures a consistent GTID position, and the `mysql.global_priv` table is excluded so users/grants are not imported (manage them with `User`/`Grant` objects instead). If a `logicalBackupTemplateRef` is configured, its spec is used as the base for this `Backup`. When [filtered replication](#filtered-replication) is enabled, the dump only includes the listed tables.
2. **Restore on every Pod.** The operator restores that single logical backup into each Pod of the cluster (primary and replicas), resetting the binary logs (`RESET MASTER`) on each one beforehand. Initialization waits until all Pods have a completed `Restore`, then cleans the `Restore` objects up.
3. **Configure replication.** Each Pod is pointed at the external MariaDB (using `binlogPort` if set, see [below](#replicating-through-a-maxscale-binlogrouter)) and replication is started from the GTID captured in the backup.

The logical backup is reused across Pods and across restarts as long as it is still valid (see [Backup validity and retention](#backup-validity-and-retention)); the operator only takes a new one when none exists or the existing one has expired.

## Scaling out

Increasing `spec.replicas` triggers a **scale out**. New replicas are always bootstrapped from a **physical backup** (it is faster than a logical restore and keeps the existing replicas as the source instead of hitting the external server):

1. The operator ensures a `PhysicalBackup` exists for the scale out, created from `replica.bootstrapFrom.physicalBackupTemplateRef`.
2. PVCs are provisioned for the new Pods (from a `VolumeSnapshot` if the template uses one, otherwise via a restore Job).
3. The physical backup is restored into the new Pods through rolling init Jobs, replication is configured against the external MariaDB and the new replicas join the cluster.

Scale out only starts when all current replicas are ready, and it can be rolled back at any point by setting `spec.replicas` back to the previous value.

## Replica recovery

Replicas can break for reasons that the operator cannot fix just by restarting replication (corrupted data, conflicting GTIDs, missing binlogs on the source, etc.). When recovery is enabled, the operator rebuilds the affected replica from scratch, one replica at a time to avoid service disruption.

### Errors that trigger a recovery

The operator inspects the last I/O and SQL error reported by each replica and classifies it:

* **Recoverable errors — recovery is triggered immediately:**
  * I/O errors: `1236` (fatal error reading binlog from master), `1945`, `1955` (requested GTID not in master's binlog), `1947` (GTID conflicts with a more recent one in the binlog), `1951` (master is missing the requested GTID).
  * SQL errors: `1062` (duplicate entry), `1032` (can't find record), `1034` (incorrect key file), `1049` (unknown database), `1146` (table doesn't exist).
* **Non-recoverable errors — recovery is never triggered** (these are transient or operational and rebuilding the replica would not help): `2003` (can't connect), `2013` (lost connection to master), `1158` (error reading communication packets), `2026` (TLS handshake failed), `1045` (access denied), `1130` (host not allowed), `1129` (host blocked), `1040` (too many connections).
* **Any other error** (not in either list): recovery is triggered only if the error persists for longer than `replica.recovery.errorDurationThreshold` (default `5m`). This avoids reacting to short-lived transient failures.

### How recovery works

For each replica that needs to be recovered, the operator tries a **physical backup first** and falls back to a **logical backup** from the external source if a physical backup cannot be produced:

1. **Ensure a physical backup is available.** The operator looks for an existing recovery `PhysicalBackup`; if it does not exist it creates one from `physicalBackupTemplateRef` with an immediate schedule.
2. **Pick a source node.** With `target: PreferReplica`, the backup is taken from a **healthy replica**: a secondary Pod that is `Ready`, has both the I/O and SQL replication threads running, and has been stable (no error transition) for at least **120 seconds**. This stability window avoids picking a replica that is only transiently healthy. If a healthy replica is found, the physical backup runs against it.
3. **Fall back to a logical backup.** For external replication, if **no healthy replica is available**, no backup Job gets launched. After **240 seconds** without a launched Job, the operator gives up on the physical backup, takes (or reuses) a logical backup **from the external MariaDB**, and restores that into the broken replica instead. The stale physical backup is then cleaned up.
4. **Rebuild the replica.** The faulty Pod and its PVC are deleted and recreated from the chosen backup (data directory is wiped first). Replication is then reconfigured against the external MariaDB.
5. **Confirm recovery.** The operator waits for the Pod to become `Ready` and for the replica to report no I/O or SQL errors before marking the replica as recovered and cleaning up the recovery Jobs/backups.

> If `bootstrapFrom` is not set, recovery cannot proceed and the operator emits a warning event and sets a recovery error on the `MariaDB` status.

### Timeouts

| Timeout | Value | Meaning |
| ------- | ----- | ------- |
| Replica stability window | `120s` (fixed) | A replica must be healthy and free of error transitions for this long before it is eligible as a physical backup source. |
| Physical backup Job launch timeout | `240s` (fixed) | If no backup Job is launched within this window (e.g. because no healthy replica is available), recovery falls back to a logical backup from the external source. |
| `errorDurationThreshold` | `5m` (configurable via `replica.recovery.errorDurationThreshold`) | How long an unclassified error must persist before triggering a recovery. Recoverable error codes trigger recovery immediately, ignoring this threshold. |

## Backup validity and retention

Both the logical and the physical backups used for external replication are validated against the **binary log retention of the source** before being reused, so the operator never restores from a backup whose binlogs are already gone (which would make replication impossible):

* The retention period is read from the source's `binlog_expire_logs_seconds` system variable (for servers older than `10.6.1`, the day-based value is converted to seconds).
* **Logical backup:** considered expired when its age (time since creation) exceeds the source's binlog retention. Expired logical backups are deleted and a fresh one is taken.
* **Physical backup:** considered expired when its last schedule time is older than `now - binlogRetention`. Expired physical backups are destroyed and re-created. If the operator cannot read `binlog_expire_logs_seconds`, it conservatively forces a new backup.
* The `maxRetention` of the generated backups is aligned with this same binlog retention period.

Additional notes:

* The backup storage size is the same as the storage size defined for the replicas.
* The backups do not include users (`mysql.global_priv` is excluded) to avoid privilege issues and conflicts, mainly with the `root` user. Use the regular `User` and `Grant` objects to manage users and privileges on the replicas.

## Filtered replication

It is possible to replicate only a subset of tables from the external MariaDB by listing them in `replicaFromExternal.filteredReplicaTables`, using the `database.table` format. This enables replicating tables across multiple schemas:

```yaml
spec:
  replication:
    enabled: true
    replica:
      bootstrapFrom:
        physicalBackupTemplateRef:
          name: physicalbackup-external
    replicaFromExternal:
      mariaDbRef:
        name: external-mariadb
        kind: ExternalMariaDB
      serverIdOffset: 30
      filteredReplicaTables:
        - db1.table1
        - db1.table2
```

When `filteredReplicaTables` is set:

* The logical backup taken from the external MariaDB only includes the listed tables (see [below](#how-the-filtered-backup-is-taken)).
* Replication is configured with a `replicate_do_table` entry for each table.
* GTID strict mode is automatically disabled, as partial replication is incompatible with it. This is equivalent to setting `replication.gtidStrictMode: false` and can still be overridden explicitly.

### How the filtered backup is taken

The logical backup is generated with `mariadb-dump`, on top of the common flags (`--single-transaction --events --routines`, `--master-data=1 --gtid` to capture the GTID position, and `--ignore-table=mysql.global_priv` so users/grants are not exported). The way the tables are selected depends on whether all the entries in `filteredReplicaTables` belong to the **same schema** or are spread across **multiple schemas**:

**Single schema** (all tables share the same database, e.g. `db1.table1`, `db1.table2`):

The operator dumps the database and explicitly lists the tables to include:

```bash
mariadb-dump ... --databases db1 --tables table1 table2
```

`--databases` (rather than the plain positional form) is used so the dump emits the database-context statements needed for a clean restore, and `--tables` restricts the dump to the listed tables. On restore, the target database is set upfront (`--database db1`).

**Multiple schemas** (tables span more than one database, e.g. `db1.table1`, `db2.table2`):

`mariadb-dump` cannot mix `--tables` with several databases, so the operator instead dumps all the involved schemas and **excludes everything that is not in the list** using dynamically built `--ignore-table` flags:

```bash
# the ignore flags are computed at backup time...
mapfile -t MARIADB_IGNORE_ARGS < <(mariadb ... -BNe "<query against information_schema>")
# ...and appended to the dump command
mariadb-dump ... --databases db1 db2 "${MARIADB_IGNORE_ARGS[@]}"
```

* The exclusion list is computed by querying `information_schema.TABLES` for every `BASE TABLE` **and** `VIEW` in the target schemas that is **not** in `filteredReplicaTables`, producing one `--ignore-table=<schema>.<table>` token per object. Views are excluded as well so the dump does not try to recreate views that reference tables left out of the backup.
* The query result is read with `mapfile` so each `--ignore-table` token is passed as a single argument. This keeps identifiers that contain spaces (e.g. `` `db2`.`my table` ``) intact instead of being word-split by the shell.
* No default database is set on restore; the dump carries the per-schema context, so the listed tables are restored into their respective databases.

## Replicating through a MaxScale Binlogrouter

If the external MariaDB is exposed behind a [MaxScale](https://mariadb.com/docs/maxscale/) using the [Binlogrouter](https://mariadb.com/docs/maxscale/maxscale-archive/mariadb-maxscale-25-08/maxscale-25-08-routers/mariadb-maxscale-25-08-maxscale-25-08-binlogrouter) to expose the binlog stream, set `binlogPort` on the `ExternalMariaDB`. The replicas will use this port to stream the binary logs (i.e. as the `CHANGE MASTER` port), while still using `port` for the rest of the operations:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: external-mariadb
spec:
  host: maxscale.example.com
  port: 3306
  binlogPort: 4000
  username: root
  passwordSecretKeyRef:
    name: mariadb
    key: password
```

## Services considerations
* Service: sends connections to any Pod, regardless of its replication status.
* PrimaryService: despite there being no real primary node on this setup (all Pods replicate from the external MariaDB), this Service is kept to provide a stable way to always send connections to the same Pod, as it could be required by some applications.
* SecondaryService: sends connections to the replica Pods that are `Ready`. A Pod that is currently being rebuilt during [recovery](#replica-recovery) (replication role `Unknown`) is excluded from the Service endpoints. Beyond that, traffic follows the Pod readiness: a Pod whose replication is broken, or that lags behind the source beyond `maxLagSeconds`, fails its readiness probe and stops receiving connections — unless `ignoreReplicationLivenessProbes` / `ignoreMaxLagSeconds` are set, in which case it keeps receiving read traffic.
