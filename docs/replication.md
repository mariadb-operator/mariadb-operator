# Replication

The operator supports provisioning and operating MariaDB clusters with semi-synchronous replication as a high availability topology. In the following sections we will cover how to manage the full lifecycle of a semi-synchronous replication cluster. 

In a replication setup, one primary server handles all write operations while one or more replica servers replicate data from the primary and can handle read operations. The semi-synchronous aspect ensures that at least one replica acknowledges the receipt of a transaction before the primary commits it.

Please refer to the [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication) for more details about replication.

## Table of contents
<!-- toc -->
<!-- /toc -->

## Provisioning

In order to provision a replication cluster, you need to configure a number of `replicas` greater than `1` and set the `replication.enabled=true` in the `MariaDB` CR:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replicas: 3
  replication:
    enabled: true
```

After applying the previous CR, the operator will provision a replication cluster with one primary and two replicas. The operator will take care of setting up replication, configuring the replication user and monitoring the replication status:

```bash
kubectl get pods
NAME                                    READY   STATUS    RESTARTS   AGE
mariadb-repl-0                          2/2     Running   0          2d19h
mariadb-repl-1                          2/2     Running   0          2d19h
mariadb-repl-2                          2/2     Running   0          2d19h
mariadb-repl-metrics-56865fff65-t72kc   1/1     Running   0          2d20h

kubectl get mariadb
NAME           READY   STATUS    PRIMARY          UPDATES                    AGE
mariadb-repl   True    Running   mariadb-repl-0   ReplicasFirstPrimaryLast   2d20h
```

As you can see, the primary can be identified in the `PRIMARY` column of the `kubectl get mariadb` output. You may also inspect the current replication status by checking the `MariaDB` CR status:

```bash
kubectl get mariadb mariadb-repl -o jsonpath="{.status.replication}" | jq
{
  "replicas": {
    "mariadb-repl-0": {
      "gtidCurrentPos": "0-10-24",
      "gtidIOPos": "0-10-24",
      "lastErrorTransitionTime": "2025-10-17T14:30:10Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true
    },
    "mariadb-repl-2": {
      "gtidCurrentPos": "0-10-24",
      "gtidIOPos": "0-10-24",
      "lastErrorTransitionTime": "2025-10-17T14:30:10Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true
    }
  },
  "roles": {
    "mariadb-repl-0": "Primary",
    "mariadb-repl-1": "Replica",
    "mariadb-repl-2": "Replica"
  }
}
```

The operator continiously monitors the replication status via [`SHOW SLAVE STATUS`](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/show/show-replica-status), taking it into account for internal operations and updating the CR status accordingly.

## Configuration

The replication settings can be customized under the `replication` section of the `MariaDB` CR. The following options are available:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replicas: 3
  replication:
    enabled: true
    gtidStrictMode: true
    waitPoint: AfterCommit
    ackTimeout: 10s
    syncBinlog: 1
```

- `gtidStrictMode`: Enables GTID strict mode. It is recommended and enabled by default. See [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#gtid_strict_mode).
- `waitPoint`: Determines whether the transaction should wait for an ACK after having synced the binlog (`AfterSync`) or after having committed to the storage engine (`AfterCommit`, the default). See [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication#rpl_semi_sync_master_wait_point).
- `ackTimeout`: ACK timeout for the replicas to acknowledge transactions to the primary. See [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication#rpl_semi_sync_master_timeout).
- `syncBinlog`: Number of events after which the binary log is synchronized to disk. See [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables#sync_binlog).


These options are used by the operator to render a replication configuration file that is applied to all nodes in the cluster. When updating any of these options, an [update of the cluster](#updates) will be triggered in order to apply the new configuration.

For replica-specific configuration options, please refer to the [replica configuration](#replica-configuration) section. Additional system variables may be configured via the `myCnf` configuration field. Refer to the [configuration documentation](./configuration.md#mycnf) for more details.

## Replica configuration

The following options are replica-specific and can be configured under the `replication.replica` section of the `MariaDB` CR:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replicas: 3
  replication:
    enabled: true
    replica:
      replPasswordSecretKeyRef:
        name: mariadb
        key: password
      gtid: CurrentPos
      connectionRetrySeconds: 10
      maxLagSeconds: 0
      syncTimeout: 10s
```

- `replPasswordSecretKeyRef`: Reference to the `Secret` key containing the password for the replication user, used by the replicas to connect to the primary. By default, a `Secret` with a random password will be created.
- `gtid`: GTID position mode to be used (`CurrentPos` and `SlavePos` allowed). See [MariaDB documentation](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/replication-statements/change-master-to#master_use_gtid). It defaults to `CurrentPos`.
- `connectionRetrySeconds`: Number of seconds that the replica will wait between connection retries. See [MariaDB documentation](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/replication-statements/change-master-to#master_connect_retry).
- `maxLagSeconds`: Maximum acceptable lag in seconds between the replica and the primary. If the lag exceeds this value, the [readiness probe](#readiness-probe) will fail and the replica will be marked as not ready. See [lagged replicas](#lagged-replicas) section for more details. It defaults to `0`, meaning that no lag is allowed.
- `syncTimeout`: Timeout for the replicas to be synced during switchover and failover operations. See the [primary switchover](#primary-switchover) and [primary failover](#primary-failover) sections for more details. It defaults to `10s`.

When updating any of these options, an [update of the cluster](#updates) will be triggered in order to apply the new configuration.

## Probes

Kubernetes probes are resolved by the agent (see [data-plane](./data_plane.md) documentation) in the replication topology, taking into account both the MariaDB and replication status. Additionally, as described in the [configuration documentation](./configuration.md#probes), probe thresholds may be tuned accordinly for a better reliability based on your environment.

In the following sub-sections we will be covering specifics about the replication topology.

#### Liveness probe

As part of the liveness probe, the agent checks that the MariaDB server is running and that the replication threads (`Slave_IO_Running` and `Slave_SQL_Running`) are both running on replicas. If any of these checks fail, the liveness probe will fail.

#### Readiness probe

The readiness probe checks that the MariaDB server is running and that the `Seconds_Behind_Master` value is within the acceptable lag range defined by the `spec.replication.replica.maxLagSeconds` configuration option. If the lag exceeds this value, the readiness probe will fail and the replica will be marked as not ready.

## Lagged replicas

A replica is considered to be lagging behind the primary when the `Seconds_Behind_Master` value reported by `SHOW SLAVE STATUS` exceeds the `spec.replication.replica.maxLagSeconds` configuration option. This results in the [readiness probe](#readiness-probe) failing for that replica, and it has the following implications:
- When using [Kubernetes `Services` for high availability](./high_availability.md#kubernetes-services), queries will not be forwarded to lagged replicas. This doesn't affect MaxScale routing.
- When taking a [physical backup](./physical_backup.md), lagged replicas will not be considered as a target for taking the backup.
- During a [primary switchover](#primary-switchover) managed by the operator, lagged replicas will block switchover operations, as all the replicas must be in sync before promoting the new primary. This doesn't affect MaxScale switchover operation.
- During a [primary failover](#primary-failover) managed by the operator, lagged replicas will not be considered as candidates to be promoted as the new primary. MaxScale failover will not consider lagged replicas either.
- During [updates](#updates), lagged replicas will block the update operation, as each of the replicas must pass the readiness probe before proceeding to the update of the next one.

## Backing up and restoring

In order to back up and restore a replication cluster, all the concepts and procedures described in the [physical backup](./physical_backup.md) documentation apply. 

Additionally, for the replication topology, the operator tracks the GTID position at the time of taking the backup, and sets this position in the `gtid_slave_pos` system variable when restoring the backup, as described in the [MariaDB documentation](https://mariadb.com/docs/server/server-usage/backup-and-restore/mariadb-backup/setting-up-a-replica-with-mariadb-backup).

Depending on the `PhysicalBackup` strategy used, the operator will track the GTID position accordingly:

#### mariadb-backup

When using `PhysicalBackup` with the `mariadb-backup` strategy, the GTID will be restored to a `mariadb-operator.info` file in the data directory, which the agent will expose to the operator via HTTP.

#### `VolumeSnapshot`

When using `PhysicalBackup` with the `VolumeSnapshot` strategy, the GTID position will be kept in a `k8s.mariadb.com/gtid` annotation in the `VolumeSnapshot` object, which later on the operator will read when restoring the backup.

> [!WARNING]
> Refrain from removing the `k8s.mariadb.com/gtid` annotation in the `VolumeSnapshot` object, as it is required for configuring the replica when restoring the backup.

## Primary switchover

You can declaratively trigger a primary switchover by updating the `spec.replication.primary.podIndex` field in the `MariaDB` CR to the index of the replica you want to promote as the new primary. For example, to promote the replica at index `1`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replicas: 3
  replication:
    enabled: true
    primary:
      podIndex: 1
```

You can also do this imperatively using `kubectl`:

```bash
kubectl patch mariadb mariadb-repl \
  --type='merge' \
  -p '{"spec":{"replication":{"primary":{"podIndex":1}}}}'
```

This will result in the `MariaDB` object reporting the following status:

```bash
kubectl get mariadb
NAME           READY   STATUS                                  PRIMARY          UPDATES                    AGE
mariadb-repl   False   Switching primary to 'mariadb-repl-1'   mariadb-repl-0   ReplicasFirstPrimaryLast   3d2h
```

The steps involved in the switchover operation are:
1. Lock the current primary using `FLUSH TABLES WITH READ LOCK` to ensure no new transactions are being processed.
2. Set the `read_only` system variable on the current primary to prevent any write operations.
3. Wait until all the replicas are in sync with the current primary. The timeout for this step can be configured via the `spec.replication.replica.syncTimeout` option. If the timeout is reached, the switchover operation will be retried from the beginning.
4. Promote the selected replica to be the new primary.
5. Connect replicas to the new primary.
6. Change the current primary to be a replica of the new primary.

If the switchover operation is stuck waiting for replicas to be in sync, you can check the `MariaDB` status to identify which replicas are causing the issue. Furthermore, if still in this step, you can cancel the switchover operation by setting back the `spec.replication.primary.podIndex` field back to the previous primary index.

Our recommendation for production environments is to rely on [MaxScale](./maxscale.md) for the [switchover operation](./maxscale.md#primary-server-switchover), as it provides [several advantages](./high_availability.md#maxscale).

## Primary failover

You can configure the operator to automatically perform a primary failover whenever the current primary becomes unavailable:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replicas: 3
  replication:
    enabled: true
    primary:
      automaticFailover: true
      automaticFailoverDelay: 0s
```
Optionally, you may also specify a `automaticFailoverDelay`, which will add a delay before triggering the failover operation. By default, the failover is immediate, but introducing a delay may be useful to avoid failovers due to transient issues.

Whenever the primary becomes unavailable, the following status will be reported in the `MariaDB` CR:

```bash
kubectl get mariadb
NAME           READY   STATUS    PRIMARY          UPDATES                    AGE
mariadb-repl   True    Running   mariadb-repl-0   ReplicasFirstPrimaryLast   3d2h

kubectl delete pod mariadb-repl-0
pod "mariadb-repl-0" deleted

kubectl get mariadb
NAME           READY   STATUS                                  PRIMARY          UPDATES                    AGE
mariadb-repl   False   Switching primary to 'mariadb-repl-1'   mariadb-repl-0   ReplicasFirstPrimaryLast 

kubectl get mariadb
NAME           READY   STATUS    PRIMARY          UPDATES                    AGE
mariadb-repl   True    Running   mariadb-repl-1   ReplicasFirstPrimaryLast   3d2h
```

The criteria for choosing a new primary is:
- The `Pod` should be in `Ready` state, therefore not considering unavailable or lagged replicas (see [readiness probe](#readiness-probe) and [lagged replicas](#lagged-replicas) sections).
- Both the `Slave_IO_Running` and `Slave_SQL_Running` threads should be running.
- The replica should not have relay log events.
- Among the candidates, the one with the highest `gtid_current_pos` will be selected.

Once the new primary is selected, the failover process will be performed, consisting of the following steps:
1. Wait for the new primary to apply all relay log events.
2. Promote the selected replica to be the new primary.
3. Connect replicas to the new primary.

Our recommendation for production environments is to rely on [MaxScale](./maxscale.md) for the failover process, as it provides [several advantages](./high_availability.md#maxscale).

## Updates

When updating a replication cluster, all the considerations and procedures described in the [updates](./updates.md) documentation apply.

Furthermore, for the replication topology, the operator will trigger an additional [switchover operation](#primary-switchover) once all the replicas have been updated, just before updating the primary. This ensures that the primary is always updated last, minimizing the impact on write operations.

The steps involved in updating a replication cluster are:
1. Update each replica one by one, waiting for each replica to be ready before proceeding to the next one (see [readiness probe](#readiness-probe) section).
2. Once all replicas are up to date and synced, perform a [primary switchover](#primary-switchover) to promote one of the replicas as the new primary. If `MariaDB` CR has a `MaxScale` configured using the `spec.maxScaleRef` field, the operator will trigger the [primary switchover in MaxScale](./maxscale.md#primary-server-switchover) instead.
3. Update the previous primary, now running as a replica.

## Scaling out

## Replica recovery

## Troubleshooting

#### Current status and events

#### Common errors