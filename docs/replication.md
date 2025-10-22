# Replication

The operator supports provisioning and operating MariaDB clusters with replication as a highly availability topology. In the following sections we will be covering how to manage the full lifecycle of a replication cluster. 

In a replication setup, one primary server handles all write operations while one or more replica servers replicate data from the primary, being able to handle read operations. More precisely, the primary has a binary log and the replicas asynchronously replicate the binary log events over the network.

Please refer to the [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication) for more details about replication.

## Table of contents
<!-- toc -->
- [Provisioning](#provisioning)
- [Asynchronous vs semi-syncrhonous replication](#asynchronous-vs-semi-syncrhonous-replication)
- [Configuration](#configuration)
- [Replica configuration](#replica-configuration)
- [Probes](#probes)
- [Lagged replicas](#lagged-replicas)
- [Backing up and restoring](#backing-up-and-restoring)
- [Primary switchover](#primary-switchover)
- [Primary failover](#primary-failover)
- [Updates](#updates)
- [Scaling out](#scaling-out)
- [Replica recovery](#replica-recovery)
- [Troubleshooting](#troubleshooting)
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
    "mariadb-repl-1": {
      "gtidCurrentPos": "0-10-155",
      "gtidIOPos": "0-10-155",
      "lastErrorTransitionTime": "2025-10-22T10:51:10Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true
    },
    "mariadb-repl-2": {
      "gtidCurrentPos": "0-10-155",
      "gtidIOPos": "0-10-155",
      "lastErrorTransitionTime": "2025-10-22T10:47:29Z",
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

## Asynchronous vs semi-syncrhonous replication

By default, [semi-synchronous replication](https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication) is configured, which requires an acknowledgement from at least one replica before committing the transaction back to the client. This trades off performance for better consistency and facilitates [failover](#primary-failover) and [switchover](#primary-switchover) operations.

If you are aiming for better performance, you can disable semi-synchronous replication, and go fully asynchronous, please refer to [configuration](#asynchronous-replication) section for doing so.

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
    semiSyncEnabled: true
    semiSyncAckTimeout: 10s
    semiSyncWaitPoint: AfterCommit
    syncBinlog: 1
```

- `gtidStrictMode`: Enables GTID strict mode. It is recommended and enabled by default. See [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#gtid_strict_mode).
- `semiSyncEnabled`: Determines whether semi-synchronous replication should be enabled. It is enabled by default. See [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication).
- `semiSyncAckTimeout`: ACK timeout for the replicas to acknowledge transactions to the primary. It requires semi-synchronous replication. See [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication#rpl_semi_sync_master_timeout).
- `semiSyncWaitPoint`: Determines whether the transaction should wait for an ACK after having synced the binlog (`AfterSync`) or after having committed to the storage engine (`AfterCommit`, the default). It requires semi-synchronous replication. See [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication#rpl_semi_sync_master_wait_point).
- `syncBinlog`: Number of events after which the binary log is synchronized to disk. See [MariaDB documentation](https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables#sync_binlog).


These options are used by the operator to create a replication configuration file that is applied to all nodes in the cluster. When updating any of these options, an [update of the cluster](#updates) will be triggered in order to apply the new configuration.

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
- `gtid`: GTID position mode to be used (`CurrentPos` and `SlavePos` allowed). It defaults to `CurrentPos`. See [MariaDB documentation](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/replication-statements/change-master-to#master_use_gtid).
- `connectionRetrySeconds`: Number of seconds that the replica will wait between connection retries. See [MariaDB documentation](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/replication-statements/change-master-to#master_connect_retry).
- `maxLagSeconds`: Maximum acceptable lag in seconds between the replica and the primary. If the lag exceeds this value, the [readiness probe](#readiness-probe) will fail and the replica will be marked as not ready. It defaults to `0`, meaning that no lag is allowed. See [lagged replicas](#lagged-replicas) section for more details. 
- `syncTimeout`: Timeout for the replicas to be synced during switchover and failover operations. It defaults to `10s`. See the [primary switchover](#primary-switchover) and [primary failover](#primary-failover) sections for more details. 

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

> [!IMPORTANT]  
> Our recommendation for production environments is to rely on [MaxScale](./maxscale.md) for the [switchover operation](./maxscale.md#primary-server-switchover), as it provides [several advantages](./high_availability.md#maxscale).

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

## Primary failover

> [!IMPORTANT]  
> Our recommendation for production environments is to rely on [MaxScale](./maxscale.md) for the failover process, as it provides [several advantages](./high_availability.md#maxscale).

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
mariadb-repl   False   Switching primary to 'mariadb-repl-1'   mariadb-repl-0   ReplicasFirstPrimaryLast   3d2h 

kubectl get mariadb
NAME           READY   STATUS    PRIMARY          UPDATES                    AGE
mariadb-repl   True    Running   mariadb-repl-1   ReplicasFirstPrimaryLast   3d2h
```

The criteria for choosing a new primary is:
- The `Pod` should be in `Ready` state, therefore not considering unavailable or lagged replicas (see [readiness probe](#readiness-probe) and [lagged replicas](#lagged-replicas) sections).
- Both the IO(`Slave_IO_Running`) and the SQL(`Slave_SQL_Running`) threads should be running.
- The replica should not have relay log events.
- Among the candidates, the one with the highest `gtid_current_pos` will be selected.

Once the new primary is selected, the failover process will be performed, consisting of the following steps:
1. Wait for the new primary to apply all relay log events.
2. Promote the selected replica to be the new primary.
3. Connect replicas to the new primary.

## Updates

When updating a replication cluster, all the considerations and procedures described in the [updates](./updates.md) documentation apply.

Furthermore, for the replication topology, the operator will trigger an additional [switchover operation](#primary-switchover) once all the replicas have been updated, just before updating the primary. This ensures that the primary is always updated last, minimizing the impact on write operations.

The steps involved in updating a replication cluster are:
1. Update each replica one by one, waiting for each replica to be ready before proceeding to the next one (see [readiness probe](#readiness-probe) section).
2. Once all replicas are up to date and synced, perform a [primary switchover](#primary-switchover) to promote one of the replicas as the new primary. If `MariaDB` CR has a `MaxScale` configured using the `spec.maxScaleRef` field, the operator will trigger the [primary switchover in MaxScale](./maxscale.md#primary-server-switchover) instead.
3. Update the previous primary, now running as a replica.

## Scaling out

Scaling out a replication cluster implies adding new replicas to the cluster i.e scaling horizontally. The process involves taking a physical backup from a ready replica to setup the new replica PVC, and upscaling the replication cluster afterwards.

The first step is to define the [`PhysicalBackup` strategy](./physical_backup.md#backup-strategies) to be used for taking the backup. For doing so, we will be defining a `PhysicalBackup` CR, that will be used by the operator as template for creating the actual `PhysicalBackup` object during scaling out events. For instance, to use the `mariadb-backup` strategy, we can define the following `PhysicalBackup`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup-tpl
spec:
  mariaDbRef:
    name: mariadb-repl
  schedule:
    suspend: true
  storage:
    s3:
      bucket: scaleout
      prefix: mariadb
      endpoint: minio.minio.svc.cluster.local:9000
      region:  us-east-1
      accessKeyIdSecretKeyRef:
        name: minio
        key: access-key-id
      secretAccessKeySecretKeyRef:
        name: minio
        key: secret-access-key
      tls:
        enabled: true
        caSecretKeyRef:
          name: minio-ca
          key: ca.crt
  timeout: 1h
  podAffinity: true
```

It is important to note that, we set the `spec.schedule.suspend=true` to prevent scheduling this backup, as it will be only be used as a template. 

Alternatively, you may also use a `VolumeSnapshot` strategy for taking the backup:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup-tpl
spec:
  mariaDbRef:
    name: mariadb-repl
  schedule:
    suspend: true
  storage:
    volumeSnapshot:
      volumeSnapshotClassName: csi-hostpath-snapclass
```

Once the `PhysicalBackup` template is created, you neeed to set a reference to it in the `spec.replication.replica.bootstrapFrom`, indicating that this will be the source for creating new replicas:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replication:
    enabled: true
    replica:
      bootstrapFrom:
        physicalBackupTemplateRef:
          name: physicalbackup-tpl
```

At this point, you can proceed to scale out the cluster by increasing the `spec.replicas` field in the `MariaDB` CR. For example, to scale out from `3` to `4` replicas:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replicas: 4
  replication:
    enabled: true
    replica:
      bootstrapFrom:
        physicalBackupTemplateRef:
          name: physicalbackup-tpl
```

You can also do this imperatively using `kubectl`:

```bash
kubectl scale mariadb mariadb-repl --replicas=4
```

This will trigger an scaling out operation, resulting in:
- A `PhysicalBackup` based on the template being created.
- Creating a new PVC for the new replica based on the `PhysicalBackup`.
- Upscaling the `StatefulSet`, adding a `Pod` that mounts the newly created PVC.
- The `Pod` is configured as a replica, connected to the primary by starting the replication in the GTID position stored in the backup.

```bash
kubectl scale mariadb mariadb-repl --replicas=4
mariadb.k8s.mariadb.com/mariadb-repl scaled

kubectl get mariadb
NAME           READY   STATUS        PRIMARY          UPDATES                    AGE
mariadb-repl   False   Scaling out   mariadb-repl-1   ReplicasFirstPrimaryLast   3d5h

kubectl get physicalbackups
NAME                                    COMPLETE   STATUS      MARIADB        LAST SCHEDULED   AGE
mariadb-repl-physicalbackup-scale-out   True       Success     mariadb-repl   14s              14s
physicalbackup-tpl                      False      Suspended   mariadb-repl                    3d8h

kubectl get pods
NAME                                    READY   STATUS    RESTARTS   AGE
mariadb-repl-0                          2/2     Running   0          137m
mariadb-repl-1                          2/2     Running   0          3d5h
mariadb-repl-2                          2/2     Running   0          3d5h
mariadb-repl-3                          2/2     Running   0          40s
mariadb-repl-metrics-56865fff65-t72kc   1/1     Running   0          3d5h

kubectl get mariadb
NAME           READY   STATUS    PRIMARY          UPDATES                    AGE
mariadb-repl   True    Running   mariadb-repl-1   ReplicasFirstPrimaryLast   3d5h
```

It is important to note that, if there are no ready replicas available at the time of the scaling out operation, the `PhysicalBackup` will not become ready, and the scaling out operation will be stuck until a replica becomes ready. You have the ability to cancel the scaling out operation by setting back the `spec.replicas` field to the previous value.

## Replica recovery

The operator has the ability to automatically recover replicas that become unavailable and report a specific error code in the replication status.  For doing so, the operator continiously monitors the replication status of each replica, and whenever a replica reports an error code listed in the table below, the operator will trigger an automated recovery process for that replica:

| Error Code | Thread | Description | Documentation |
|------------|--------|-------------|---------------|
| 1236       | IO     | Error 1236: Got fatal error from master when reading data from binary log. | [MariaDB docs](https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1200-to-1299/e1236) |

To perform the recovery, the operator will take a physical backup from a ready replica, restore it to the failed replica PVC, and reconfigure the replica to connect to the primary from the GTID position stored in the backup.

Similarly to the [scaling out](#scaling-out) operation, you need to define a `PhysicalBackup` template and set a reference to it in the `spec.replication.replica.bootstrapFrom` field of the `MariaDB` CR. Additionally, you need to explicitly enable the replica recovery, as it is disabled by default:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replication:
    enabled: true
    replica:
      bootstrapFrom:
        physicalBackupTemplateRef:
          name: physicalbackup-tpl
      recovery:
        enabled: true
        errorDurationThreshold: 5m
```

The `errorDurationThreshold` option defines the duration after which, a replica reporting an unknown error code will be considered for recovery. This is useful to avoid recovering replicas due to transient issues. It defaults to `5m`.

We will be simulating a `1236` error in a replica to demostrate how the recovery process works:

> [!CAUTION]
> Do not perform the following steps in a production environment.

-  Purge the binary logs in the primary:
```bash
PRIMARY=$(kubectl get mariadb mariadb-repl -o jsonpath="{.status.currentPrimary}")
echo "Purging binary logs in primary $PRIMARY"
kubectl exec -it $PRIMARY -c mariadb -- mariadb -u root -p'MariaDB11!' --ssl=false -e "FLUSH LOGS; PURGE BINARY LOGS BEFORE NOW();"
```

- Delete the PVC and restart one of the replicas. If this is a brand new `MariaDB` instance, you might need to re-attempt this step:
```bash
REPLICA=$(kubectl get mariadb mariadb-repl -o jsonpath='{.status.replication.replicas}' | jq -r 'keys[]' | head -n1)
echo "Deleting PVC and restarting replica $REPLICA"
kubectl delete pvc storage-$REPLICA --wait=false 
kubectl delete pod $REPLICA --wait=false 
```

This will trigger a replica recovery operation, resulting in:
- A `PhysicalBackup` based on the template being created.
- Restoring the backup to the failed replica PVC.
- Reconfigure the replica to connect to the primary from the GTID position stored in the backup.

```bash
kubectl get mariadb
NAME           READY   STATUS                PRIMARY          UPDATES                    AGE
mariadb-repl   False   Recovering replicas   mariadb-repl-1   ReplicasFirstPrimaryLast   3d6h

kubectl get physicalbackups
NAME                                           COMPLETE   STATUS      MARIADB        LAST SCHEDULED   AGE
mariadb-repl-physicalbackup-replica-recovery   True       Success     mariadb-repl   31s              31s
physicalbackup-tpl                             False      Suspended   mariadb-repl                    3d9h

kubectl get pods
NAME                                                              READY   STATUS            RESTARTS       AGE
mariadb-repl-0                                                    0/2     PodInitializing   0              22s
mariadb-repl-0-physicalbackup-init-qn79f                          0/1     Completed         0              8s
mariadb-repl-1                                                    2/2     Running           0              3d6h
mariadb-repl-2                                                    2/2     Running           0              3d6h
mariadb-repl-metrics-56865fff65-t72kc                             1/1     Running           0              3d6h
mariadb-repl-physicalbackup-replica-recovery-2025102020270r98zr   0/1     Completed         0              31s

kubectl get mariadb
NAME           READY   STATUS    PRIMARY          UPDATES                    AGE
mariadb-repl   True    Running   mariadb-repl-1   ReplicasFirstPrimaryLast   3d6h
```

It is important to note that, if there are no ready replicas available at the time of the recovery operation, the `PhysicalBackup` will not become ready, and the recovery operation will be stuck until a replica becomes ready. You have the ability to cancel the recovery operation by setting `spec.replication.replica.recovery.enabled=false`.

## Troubleshooting

The operator tracks the current replication status under the `MariaDB` status subresource. This status is updated every time the operator reconciles the `MariaDB` resource, and it is the first place to look for when troubleshooting replication issues:

```bash
kubectl get mariadb mariadb-repl -o jsonpath="{.status.replication}" | jq
{
  "replicas": {
    "mariadb-repl-1": {
      "gtidCurrentPos": "0-10-155",
      "gtidIOPos": "0-10-155",
      "lastErrorTransitionTime": "2025-10-22T10:51:10Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true
    },
    "mariadb-repl-2": {
      "gtidCurrentPos": "0-10-155",
      "gtidIOPos": "0-10-155",
      "lastErrorTransitionTime": "2025-10-22T10:47:29Z",
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

Additionally, also under the status subresource, the operator sets status conditions whenever a specific state of the `MariaDB` lifecycle is reached:

```bash
kubectl get mariadb mariadb-repl -o jsonpath="{.status.conditions}" | jq
[
  {
    "lastTransitionTime": "2025-10-20T20:28:09Z",
    "message": "Running",
    "reason": "StatefulSetReady",
    "status": "True",
    "type": "Ready"
  },
  {
    "lastTransitionTime": "2025-10-17T14:17:43Z",
    "message": "Updated",
    "reason": "Updated",
    "status": "True",
    "type": "Updated"
  },
  {
    "lastTransitionTime": "2025-10-17T14:17:58Z",
    "message": "Replication configured",
    "reason": "ReplicationConfigured",
    "status": "True",
    "type": "ReplicationConfigured"
  },
  {
    "lastTransitionTime": "2025-10-20T17:14:38Z",
    "message": "Switchover complete",
    "reason": "SwitchPrimary",
    "status": "True",
    "type": "PrimarySwitched"
  },
  {
    "lastTransitionTime": "2025-10-20T19:31:29Z",
    "message": "Scaled out",
    "reason": "ScaledOut",
    "status": "True",
    "type": "ScaledOut"
  },
  {
    "lastTransitionTime": "2025-10-20T20:27:41Z",
    "message": "Replica recovered",
    "reason": "ReplicaRecovered",
    "status": "True",
    "type": "ReplicaRecovered"
  }
]
``` 

The operator also emits Kubernetes events during failover/switchover operations. You may check them to see how these operations progress:

```bash
kubectl get events --field-selector involvedObject.name=mariadb-repl --sort-by='.lastTimestamp'

LAST SEEN   TYPE     REASON             OBJECT                 MESSAGE
17s         Normal   PrimaryLock        mariadb/mariadb-repl   Locking primary with read lock
17s         Normal   PrimaryReadonly    mariadb/mariadb-repl   Enabling readonly mode in primary
17s         Normal   ReplicaSync        mariadb/mariadb-repl   Waiting for replicas to be synced with primary
17s         Normal   PrimaryNew         mariadb/mariadb-repl   Configuring new primary at index '0'
7s          Normal   ReplicaConn        mariadb/mariadb-repl   Connecting replicas to new primary at '0'
7s          Normal   PrimaryToReplica   mariadb/mariadb-repl   Unlocking primary '1' and configuring it to be a replica. New primary at '0'
7s          Normal   PrimaryLock        mariadb/mariadb-repl   Unlocking primary
7s          Normal   PrimarySwitched    mariadb/mariadb-repl   Primary switched from index '1' to index '0'
``` 

#### Common errors

##### Primary has purged binary logs, unable to configure replica

The primary may purge binary log events at some point, after then, if a replica requests events before that point, it will fail with the following error:

```bash
Error 1236: Got fatal error from master when reading data from binary log.
```

This is a something the operator is able to recover from, please refer to the [replica recovery section](#replica-recovery).

##### Scaling out/recovery operation stucked

These operations rely on a `PhysicalBackup` for setting up the new replicas. If this `PhysicalBackup` does not become ready, the operation will not progress. In order to debug this please refer to the [`PhysicalBackup` troubleshooting section](./physical_backup.md#troubleshooting).

One of the reasons could be that there are not replicas in ready state at the time of creating the `PhysicalBackup`, for instance, all the replicas are lagging behind the primary. Please verify that this is the case by checking the status of your `MariaDB` resource and your `Pods`.