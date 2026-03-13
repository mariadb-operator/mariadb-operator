# Point-In-Time-Recovery

Point-in-time recovery (PITR) is a feature that allows you to restore a MariaDB instance to a specific point in time. For achieving this, it combines a full base backup and the binary logs that record all changes made to the database after the backup. This is something fully automated by operator, covering archival and restoration up to a specific time, ensuring business continuity and reduced RTO and RPO.


## Table of contents

<!-- toc -->
- [Supported MariaDB versions and topologies](#supported-mariadb-versions-and-topologies)
- [Storage types](#storage-types)
- [Configuration](#configuration)
- [Full base backup](#full-base-backup)
- [Archival](#archival)
- [Binary log size](#binary-log-size)
- [Compression](#compression)
- [Server-Side Encryption with Customer-Provided Keys (SSE-C) For S3](#server-side-encryption-with-customer-provided-keys-sse-c-for-s3)
- [Binlog inventory](#binlog-inventory)
- [Binlog timeline and last recoverable time](#binlog-timeline-and-last-recoverable-time)
- [Point-in-time restoration](#point-in-time-restoration)
- [Strict mode](#strict-mode)
- [Staging storage](#staging-storage)
- [Limitations](#limitations)
- [Troubleshooting](#troubleshooting)
<!-- /toc -->


## Supported MariaDB versions and topologies

The operator uses [mariadb-binlog](https://mariadb.com/docs/server/clients-and-utilities/logging-tools/mariadb-binlog) to replay binary logs, in particular, it filters binlog events by passing a GTID to mariadb-binlog via the [`--start-position`](https://mariadb.com/docs/server/clients-and-utilities/logging-tools/mariadb-binlog/mariadb-binlog-options#j-pos-start-position-pos) flag. This is only supported by __MariaDB server 10.8 and later__, so make sure you are using a compatible MariaDB version.

Regarding supported MariaB topologies, at the moment, binary log archiving and point-in-time recovery are only supported by the __[asynchronous replication topology](./replication.md)__, which already relies on the binary logs for replication. Galera and standalone topologies will be supported in upcoming releases.

## Storage types

Full base backups and binary logs can be stored in the following object storage types:

- **S3 compatible storage**: Such as [AWS S3](https://aws.amazon.com/s3/) or [Minio](https://github.com/minio/minio).
- **[Azure Blob Storage](https://azure.microsoft.com/en-us/products/storage/blobs)**.

For additional details on configuring storage, please refer to the __[storage types](./physical_backup.md#storage-types)__ section in the physical backup documentation, same settings are applicable to the `PointInTimeRecovery` object.

## Configuration

To be able to perform a point-in-time restoration, a physical backup should be configured as full base backup. For example, you can configure a nightly backup:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup-daily
spec:
  mariaDbRef:
    name: mariadb-repl
  schedule:
    cron: "0 0 * * *"
    suspend: false
    immediate: true
  compression: bzip2
  maxRetention: 720h 
  storage:
    s3:
      bucket: physicalbackups
      prefix: mariadb
      endpoint: minio.minio.svc.cluster.local:9000
      region: us-east-1
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
```
Refer to the [full base backup](#full-base-backup) section for additional details on how to configure the full base backup.

Next step is configuring common aspects of both binary log archiving and point-in-time restoration by defining a `PointInTimeRecovery` object:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PointInTimeRecovery
metadata:
  name: pitr
spec:
  physicalBackupRef:
    name: physicalbackup-daily
  storage:
    s3:
      bucket: binlogs
      prefix: mariadb
      endpoint: minio.minio.svc.cluster.local:9000
      region: us-east-1
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
  compression: gzip
  archiveTimeout: 1h
  strictMode: false
``` 

- `physicalBackupRef`: It is a reference to the `PhysicalBackup`  resource used as full base backup. See [full base backup](#full-base-backup).
- `storage`: Object storage configuration for binary logs. See [storage types](#storage-types).
- `compression`: Algorithm to be used for compressing binary logs. It is disabled by default. See [compression](#compression).
- `archiveTimeout`: Maximum duration for the binary log archival. If exceeded, agent will return an error and archival will be retried in the next archive cycle. Defaults to 1h.
- `strictMode`: Controls the behavior when a point-in-time restoration cannot reach the exact target time. It is disabled by default. See [strict mode](#strict-mode).

With this configuration in place, you can enable binary log archival in a `MariaDB` instance by setting a reference to the `PointInTimeRecovery` object:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  storage:
    size: 1Gi
  replicas: 3
  replication:
    enabled: true
  # sidecar agent will archive binary logs to the configured storage.
  pointInTimeRecoveryRef:
    name: pitr
```

Once a full base backup has been completed and the binary logs have been archived, you can perform a point-in-time restoration. For example, you can create a new `MariaDB` instance with the following configuration:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  storage:
    size: 1Gi
  replicas: 3
  replication:
    enabled: true
  # bootstrap the instance from PITR: restore closest physical backup and replay binary logs up to targetRecoveryTime.
  bootstrapFrom:
    pointInTimeRecoveryRef:
      name: pitr
    targetRecoveryTime: 2026-02-20T18:00:04Z
```

Refer to the [point-in-time restoration](#point-in-time-restoration) section for additional details.

## Full base backup

To enable point-in-time recovery, a `PhysicalBackup` resource should be configured as full base backup. The backup should be a complete snapshot of the database at a specific point in time, and it will serve as the starting point for replaying the binary logs. Any of the supported [backup strategies](./physical_backup.md#backup-strategies) can be used as full base backup, as all of them provide a consistent snapshot of the database and a starting GTID position.

It is very important to note that a full physical backups should be completed before a point-in-time restoration can be performed. This is something that the operator accounts for when computing the [last recoverable time](#binlog-timeline-and-last-recoverable-time). 

To further expand the [last recoverable time](#binlog-timeline-and-last-recoverable-time), you can schedule an on-demand physical backup or rely on the cron scheduling for doing so. Refer to the [physical backup scheduling](./physical_backup.md#scheduling) docs for further details, see a `PhysicalBackup` resource example:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  schedule:
    cron: "0 0 * * *"
    onDemand: "1"
```

The backup taken in the new primary will establish a baseline for a new [binlog timeline](#binlog-timeline-and-last-recoverable-time), which will be expanded when new binary logs are archived.

## Archival

The mariadb-operator [sidecar agent](./data_plane.md#agent-sidecar) will periodically check for new binary logs and archive them to the configured object storage. The archival process is performed on the primary `Pod` in the asynchronous replication topology, you may check the logs of the agent sidecar container, Kubernetes events and status of the `MariaDB` objects to monitor the current status of the archival process:

```bash
kubectl logs -l k8s.mariadb.com/role=primary -c agent --tail 20
{"level":"info","ts":1773229710.7131367,"logger":"binlog-archival","msg":"Archiving binary logs"}
{"level":"info","ts":1773229710.8043964,"logger":"binlog-archival","msg":"Detected server_id change. Resetting binary log archival status...","server-id":0,"new-server-id":10}
{"level":"info","ts":1773229710.8404129,"logger":"binlog-archival.uploader","msg":"Uploading binary log","binlog":"mariadb-repl-bin.000001","object":"server-10/mariadb-repl-bin.000001.gz","start-time":"2026-03-11T11:48:30Z"}
{"level":"info","ts":1773229710.840511,"logger":"binlog-archival.uploader","msg":"Compressing binary log","binlog":"mariadb-repl-bin.000001","object":"server-10/mariadb-repl-bin.000001.gz","start-time":"2026-03-11T11:48:30Z"}
{"level":"info","ts":1773229710.8584251,"logger":"binlog-archival.uploader","msg":"Binary log uploaded","binlog":"mariadb-repl-bin.000001","object":"server-10/mariadb-repl-bin.000001.gz","start-time":"2026-03-11T11:48:30Z","total-time":"18.037438ms"}
{"level":"info","ts":1773229710.858639,"logger":"binlog-archival","msg":"Binary log mariadb-repl-bin.000001 archived"}
{"level":"info","ts":1773229710.8730743,"logger":"binlog-archival.uploader","msg":"Uploading binary log","binlog":"mariadb-repl-bin.000002","object":"server-10/mariadb-repl-bin.000002.gz","start-time":"2026-03-11T11:48:30Z"}
{"level":"info","ts":1773229710.8731706,"logger":"binlog-archival.uploader","msg":"Compressing binary log","binlog":"mariadb-repl-bin.000002","object":"server-10/mariadb-repl-bin.000002.gz","start-time":"2026-03-11T11:48:30Z"}
{"level":"info","ts":1773229710.8925226,"logger":"binlog-archival.uploader","msg":"Binary log uploaded","binlog":"mariadb-repl-bin.000002","object":"server-10/mariadb-repl-bin.000002.gz","start-time":"2026-03-11T11:48:30Z","total-time":"19.470862ms"}
{"level":"info","ts":1773229710.89273,"logger":"binlog-archival","msg":"Binary log mariadb-repl-bin.000002 archived"}
{"level":"info","ts":1773229710.9050276,"logger":"binlog-archival.uploader","msg":"Uploading binary log","binlog":"mariadb-repl-bin.000003","object":"server-10/mariadb-repl-bin.000003.gz","start-time":"2026-03-11T11:48:30Z"}
{"level":"info","ts":1773229710.905098,"logger":"binlog-archival.uploader","msg":"Compressing binary log","binlog":"mariadb-repl-bin.000003","object":"server-10/mariadb-repl-bin.000003.gz","start-time":"2026-03-11T11:48:30Z"}
{"level":"info","ts":1773229710.9248638,"logger":"binlog-archival.uploader","msg":"Binary log uploaded","binlog":"mariadb-repl-bin.000003","object":"server-10/mariadb-repl-bin.000003.gz","start-time":"2026-03-11T11:48:30Z","total-time":"19.873411ms"}
{"level":"info","ts":1773229710.9251468,"logger":"binlog-archival","msg":"Binary log mariadb-repl-bin.000003 archived"}
{"level":"info","ts":1773229710.925272,"logger":"binlog-archival","msg":"Binlog archival done"}

kubectl get events --field-selector involvedObject.name=mariadb-repl
TYPE     REASON           OBJECT                 MESSAGE
Normal    BinlogArchived         mariadb/mariadb-repl               Binary log mariadb-repl-bin.000001 archived
Normal    BinlogArchived         mariadb/mariadb-repl               Binary log mariadb-repl-bin.000002 archived
Normal    BinlogArchived         mariadb/mariadb-repl               Binary log mariadb-repl-bin.000003 archived

kubectl get mariadb mariadb-repl -o jsonpath='{.status.pointInTimeRecovery}' | jq
{
  "lastArchivedBinaryLog": "mariadb-repl-bin.000003",
  "lastArchivedGtid": "0-10-1000",
  "lastArchivedPosition": 678,
  "lastArchivedTime": "2026-03-11T11:55:31Z",
  "serverId": 10,
  "storageReadyForArchival": true
}
```

There are a couple of important considerations regarding binary log archival:
- The archival process should start from a clean state, which means that the object storage should be empty at the time of the first archival.
- If the archival process fails (e.g., due to network issues or storage unavailability), it will be retried in the next archive cycle.
- Manually executing [`PURGE BINARY LOGS`](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/purge-binary-logs)command on the database is not recommended, as it can lead to inconsistencies between the database and the archived binary logs.
- Manually executing [`FLUSH BINARY LOGS`](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/flush-commands/flush) command on the database should be compatible with the archival process, it will force the active binary log to be closed and will be archived by the agent in the next archive cycle.

## Binary log size

The server has a default [`max_binlog_size`](https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables#max_binlog_size) of 1GB, which means that a new binary log file will be created once the current one reaches that size. This is sensible default value for most cases, but it can be adjusted based on the data volume in order to enable a faster archival, and therefore a reduced RPO:

| Environment | Recommended Size | Rationale |
|-------------|------------------|-----------|
| Low Traffic | 128MB | Keeps file size minimal for slow-growing logs. |
| Standard | 256MB | Balances rotation frequency with server overhead.  |
| High Throughput | 512MB - 1GB | Reduces the contention caused by frequent rotations in write-heavy environments. |

The smaller the binlog file size, the more frequently the files will be rotated and archived, which can lead to increased load on the database `Pod` and the storage system. On the other hand, setting a very high binlog file size can lead to longer archival times and increased RPO.

Refer to  the [configuration](./configuration.md#mycnf) documentation for instructions on how to set the `max_binlog_size` server variable in the `MariaDB` instance.

## Compression

In order to reduce storage usage and save bandwidth during archival and restoration, the operator supports compressing the binary log files. Compression is enabled by setting the `compression` field in the `PointInTimeRecovery` configuration:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PointInTimeRecovery
metadata:
  name: pitr
spec:
  compression: gzip
```

The supported compression algorithms are:
- `bzip2`: Good compression ratio, but slower compression/decompression speed compared to gzip.
- `gzip`: Good compression/decompression speed, but worse compression ratio compared to bzip2.
- `none`: No compression.

Compression is disabled by default, and the are some important considerations before enabling it:
- Compression is immutable, which means that once configured and binary logs have been archived with a specific algorithm, it cannot be changed. This also applies to restoration, the same compression algorithm should be configured as the one used for archival.
- Although it saves storage space and bandwidth, the restoration process may take longer when compression is enabled, leading to an increased RTO.

## Server-Side Encryption with Customer-Provided Keys (SSE-C) For S3

When using S3-compatible storage, you can enable server-side encryption using your own encryption key (SSE-C) by providing a reference to a `Secret` containing a 32-byte (256-bit) key encoded in base64:

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: ssec-key
stringData:
  # 32-byte key encoded in base64 (use: openssl rand -base64 32)
  customer-key: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
```

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PointInTimeRecovery
metadata:
  name: pitr
spec:
  physicalBackupRef:
    name: physicalbackup-daily
  storage:
    s3:
      bucket: binlogs
      endpoint: minio.minio.svc.cluster.local:9000
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
      ssec:
        customerKeySecretKeyRef:
          name: ssec-key
          key: customer-key
```

> [!IMPORTANT]  
> When using SSE-C, you are responsible for managing and securely storing the encryption key. If you lose the key, you will not be able to decrypt your binary logs. Ensure you have proper key management procedures in place.

> [!NOTE]  
> When replaying SSE-C encrypted binary logs via `bootstrapFrom`, the same key must be provided in the S3 configuration.

## Binlog inventory

The operator maintains an inventory of the archived binary logs in an `index.yaml` file located at the root of the configured object storage. This file contains a list of all the archived binary logs per each server, along with their GTIDs and other metadata utilized internally. Here is an example of the `index.yaml` file:

```yaml
apiVersion: v1
binlogs:
  server-10:
  ...
  - binlogFilename: mariadb-repl-bin.000003
    binlogVersion: 4
    firstGtid: 0-10-527
    firstTime: "2026-02-27T16:03:22Z"
    lastGtid: 0-10-1041
    lastTime: "2026-02-27T16:03:50Z"
    logPosition: 268493636
    previousGtids:
    - 0-10-526
    rotateEvent: true
    serverId: 10
    serverVersion: 11.8.5-2-MariaDB
    stopEvent: false
  - binlogFilename: mariadb-repl-bin.000004
    binlogVersion: 4
    firstGtid: 0-10-1042
    firstTime: "2026-02-27T16:03:50Z"
    lastGtid: 0-10-1559
    lastTime: "2026-02-27T16:04:15Z"
    logPosition: 268506819
    previousGtids:
    - 0-10-1041
    rotateEvent: true
    serverId: 10
    serverVersion: 11.8.5-2-MariaDB
    stopEvent: false
```

This file is used internally by the operator to keep track of the archived binary logs, and it is updated after each successful archival. It should not be modified manually, as it can lead to inconsistencies between the actual archived binary logs and the inventory.

When it comes to point-in-time restoration, this file serves as a source of truth to compute the [binlog timeline and the last recoverable time](#binlog-timeline-and-last-recoverable-time).

## Binlog timeline and last recoverable time

Taking into account the last completed physical backup GTID and the archived binlogs in the [inventory](#binlog-inventory), the operator computes a timeline of binary logs that can replayed and its corresponding last recoverable time. The last recoverable time is the latest timestamp that the `MariaDB` instance can be restored to. This information is crucial for understanding the RPO of the system and for making informed decisions during a recovery process.

You can easily check the [last recoverable time](#binlog-timeline-and-last-recoverable-time) by looking at the status of the `PointInTimeRecovery` object:

```bash
kubectl get pitr
NAME   PHYSICAL BACKUP        LAST RECOVERABLE TIME   STRICT MODE   AGE
pitr   physicalbackup-daily   2026-02-27T20:10:42Z    true          43h
```

Then, you may provide exactly this timestamp, or an earlier one, as target recovery time when bootstrapping a new `MariaDB` instance, as described in the [point-in-time restoration](#point-in-time-restoration) section.

## Point-in-time restoration

In order to perform a point-in-time restoration, you can create a new `MariaDB` instance with a reference to the `PointInTimeRecovery` object in the `bootstrapFrom` field, along with the `targetRecoveryTime` field indicating the desired point-in-time to restore to.

For setting the `targetRecoveryTime`, it is recommended to check the last recoverable time first in the `PointInTimeRecovery` object:

```bash
kubectl get pitr
NAME   PHYSICAL BACKUP        LAST RECOVERABLE TIME   STRICT MODE   AGE
pitr   physicalbackup-daily   2026-02-27T20:10:42Z    true          43h
```

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
  storage:
    size: 1Gi
  replicas: 3
  replication:
    enabled: true
  # bootstrap the instance from PITR: restore closest physical backup and replay binary logs up to targetRecoveryTime.
  bootstrapFrom:
    pointInTimeRecoveryRef:
      name: pitr
    targetRecoveryTime: 2026-02-20T18:00:04Z
    restoreJob:
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          memory: 1Gi
    logLevel: debug
```

- `pointInTimeRecoveryRef`: Reference to the `PointInTimeRecovery` object that contains the configuration for the point-in-time recovery.
- `targetRecoveryTime`: The desired point in time to restore to. It should be in RFC3339 format. If not provided, the current time will be used as target recovery time, which means restoring up to the [last recoverable time](#binlog-timeline-and-last-recoverable-time).
- `restoreJob`: Compute resources and metadata configuration for the restoration job. To reduce RTO, it is recommended to properly tune compute resources.
- `logLevel`: Log level for the operator container, part of the restoration job.

The restoration process will match the closest physical backup before or at the `targetRecoveryTime`, and then it will replay the archived binary logs from the backup GTID position up until the `targetRecoveryTime`:

```bash
kubectl apply -f mariadb_replication_pitr_s3.yaml
mariadb.k8s.mariadb.com/mariadb-repl created

kubectl get mariadb
NAME           READY   STATUS         PRIMARY          UPDATES                    AGE
mariadb-repl   False   Initializing   mariadb-repl-0   ReplicasFirstPrimaryLast   40s

kubectl get pods
NAME                           READY   STATUS      RESTARTS       AGE
mariadb-repl-0                 2/2     Running     0              36s
mariadb-repl-0-pb-init-gp4gl   0/1     Completed   0              45s
mariadb-repl-1                 1/2     Running     0              15s
mariadb-repl-1-pb-init-z44d7   0/1     Completed   0              27s
mariadb-repl-2-pb-init-qmkcv   0/1     Completed   0              8s

kubectl get mariadb
NAME           READY   STATUS              PRIMARY          UPDATES                    AGE
mariadb-repl   False   Replaying binlogs   mariadb-repl-0   ReplicasFirstPrimaryLast   93s

kubectl get pods
NAME                          READY   STATUS      RESTARTS       AGE
mariadb-repl-0                2/2     Running     0              84s
mariadb-repl-1                2/2     Running     0              58s
mariadb-repl-2                2/2     Running     0              38s
mariadb-repl-pitr-pj6fr       0/1     Init:0/1    0              8s

kubectl logs mariadb-repl-pitr-pj6fr -c mariadb-operator
{"level":"info","ts":1772294432.9904623,"msg":"Starting point-in-time recovery"}
{"level":"info","ts":1772294432.9907954,"msg":"Getting binlog index from object storage"}
{"level":"info","ts":1772294432.9951825,"msg":"Building binlog timeline"}
{"level":"info","ts":1772294432.9952044,"logger":"binlog-timeline","msg":"Building binlog timeline","num-binlogs":0,"start-gtid":"0-10-4","target-time":"2026-02-27T21:10:42+01:00","strict-mode":false,"server":"server-10"}
{"level":"info","ts":1772294432.9952517,"msg":"Got binlog timeline","path":["server-10/mariadb-repl-bin.000002","server-10/mariadb-repl-bin.000003","server-10/mariadb-repl-bin.000004","server-10/mariadb-repl-bin.000005"]}
{"level":"info","ts":1772294432.9952574,"msg":"Pulling binlogs into staging area","staging-path":"/binlogs","compression":"gzip"}
{"level":"info","ts":1772294432.9952772,"logger":"storage","msg":"Pulling binlog","binlog":"server-10/mariadb-repl-bin.000005","start-time":"2026-02-28T16:00:32Z"}
{"level":"info","ts":1772294432.9967375,"logger":"storage","msg":"Decompressing binlog","binlog":"server-10/mariadb-repl-bin.000005","start-time":"2026-02-28T16:00:32Z","compressed-file":"server-10/mariadb-repl-bin.000005.gz","decompressed-file":"/binlogs/server-10/mariadb-repl-bin.000005","compression":"gzip"}
{"level":"info","ts":1772294437.3718772,"msg":"Binlogs pulled into staging area","staging-path":"/binlogs","compression":"gzip"}
{"level":"info","ts":1772294437.3719199,"msg":"Writing target file","file-path":"/binlogs/0-binlog-target.txt"}
```

As you can see, the restoration process includes the following steps:
1. Perform a rolling restore of the [full base backup](#full-base-backup), one `Pod` at a time.
2. Configure replication in the `MariaDB` instance.
3. Get the base backup GTID, to be used as the starting point for replaying the binary logs.
4. Schedule the point-in-time restoration job, which will:
   1. Build the [binlog timeline](#binlog-timeline-and-last-recoverable-time) based on the base backup GTID and the [archived binary log inventory](#binlog-inventory).
   2. Pull the binary logs in the timeline into a [staging area](#staging-storage).
   3. Replay the binary logs using [mariadb-binlog](https://mariadb.com/docs/server/clients-and-utilities/logging-tools/mariadb-binlog) from the GTID position of the base backup up to the `targetRecoveryTime`.

After having completed the restoration process, the following status conditions will be available for you to inspect the restoration process:

```bash
kubectl get mariadb mariadb-repl -o jsonpath='{.status.conditions}' | jq
[
  {
    "lastTransitionTime": "2026-03-01T12:15:06Z",
    "message": "Initialized",
    "reason": "Initialized",
    "status": "True",
    "type": "Initialized"
  },
  {
    "lastTransitionTime": "2026-03-01T12:15:06Z",
    "message": "Restored physical backup",
    "reason": "RestorePhysicalBackup",
    "status": "True",
    "type": "BackupRestored"
  },
  {
    "lastTransitionTime": "2026-03-01T12:15:06Z",
    "message": "Replication configured",
    "reason": "ReplicationConfigured",
    "status": "True",
    "type": "ReplicationConfigured"
  },
  {
    "lastTransitionTime": "2026-03-01T12:16:40Z",
    "message": "Replayed binlogs",
    "reason": "ReplayBinlogs",
    "status": "True",
    "type": "BinlogsReplayed"
  },
]
```

## Strict mode

The strict mode controls whether the target recovery time provided during the bootstrap process should be strictly met or not. This is configured via the `strictMode` field in the `PointInTimeRecovery` configuration, and it is disabled by default:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PointInTimeRecovery
metadata:
  name: pitr
spec:
  strictMode: true
```

When strict mode is enabled (recommended), if the target recovery time cannot be met, the initialization process will return an error early, and the `MariaDB` instance will not be created. This can happen, for example, if the target recovery time is later than the [last recoverable time](#binlog-timeline-and-last-recoverable-time). Let's assume strict mode is enabled and the last recoverable time is:

```bash
kubectl get pitr
NAME   PHYSICAL BACKUP        LAST RECOVERABLE TIME   STRICT MODE   AGE
pitr   physicalbackup-daily   2026-02-27T20:10:42Z    true          43h
```

If we attempt to provision the following `MariaDB` instance:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
  storage:
    size: 1Gi
  replicas: 3
  replication:
    enabled: true
  bootstrapFrom:
    pointInTimeRecoveryRef:
      name: pitr
    targetRecoveryTime: 2026-02-28T20:10:42Z
```

The following errors will be returned, as the target recovery time `2026-02-28T20:10:42Z` is later than the last recoverable time `2026-02-27T20:10:42Z`:

```bash
kubectl get events --field-selector involvedObject.name=mariadb-repl
LAST SEEN   TYPE      REASON                 OBJECT                     MESSAGE
41s         Warning   MariaDBInitError       mariadb/mariadb-repl       Unable to init MariaDB: target recovery time 2026-02-28 21:10:42 +0100 CET is after latest recoverable time 2026-02-27 20:10:42 +0000 UTC

kubectl get mariadb
NAME           READY   STATUS                                                                                                                          PRIMARY          UPDATES                    AGE
mariadb-repl   False   Init error: target recovery time 2026-02-28 21:10:42 +0100 CET is after latest recoverable time 2026-02-27 20:10:42 +0000 UTC   mariadb-repl-0   ReplicasFirstPrimaryLast   65s
``` 


When strict mode is disabled (default), and  the target recovery time cannot be met, the `MariaDB` provisioning will proceed and the last recoverable time will be used. This would mean that, the `MariaDB` instance will be provisioned with a recovery time of `2026-02-27T20:10:42Z`, which is the last recoverable time:

```bash
kubectl get pitr
NAME   PHYSICAL BACKUP        LAST RECOVERABLE TIME   STRICT MODE   AGE
pitr   physicalbackup-daily   2026-02-27T20:10:42Z    false         43h
```

After setting `strictMode=false`, if we attempt to create the same `MariaDB` instance as before, it will be successfully provisioned, but with a recovery time of `2026-02-27T20:10:42Z` will be used instead of the requested `2026-02-28T20:10:42Z`.

It is important to note that the last recoverable time is stored in the status field of the `PointInTimeRecovery` object, therefore if this object is deleted and recreated, the last recoverable time metadata will be lost, and it will not be available until recomputed. When it comes to restore, this implies that the error will be returned later in the process, when computing the binary log timeline, but the strict mode behaviour still applies. This is the error returned for that scenario:


```bash
kubectl get events --field-selector involvedObject.name=mariadb-repl
LAST SEEN   TYPE      REASON                 OBJECT                     MESSAGE
12s         Warning   BinlogTimelineInvalid   mariadb/mariadb-repl      Invalid binary log timeline: error getting binlog timeline between GTID 0-10-4 and target time 2026-02-28T21:10:42+01:00: timeline did not reach target time: 2026-02-28T21:10:42+01:00, last recoverable time: 2026-02-27T21:10:42+01:00

kubectl get mariadb
NAME           READY   STATUS                                                                                                                                                                                                                                                               PRIMARY          UPDATES                    AGE
mariadb-repl   False   Error replaying binlogs: Invalid binary log timeline: error getting binlog timeline between GTID 0-10-4 and target time 2026-02-28T21:10:42+01:00: timeline did not reach target time: 2026-02-28T21:10:42+01:00, last recoverable time: 2026-02-27T21:10:42+01:00   mariadb-repl-0   ReplicasFirstPrimaryLast   3m28s
``` 

## Staging storage

The operator uses a staging area to temporarily store the binary logs during the restoration process. By default, the staging area is an [`emptyDir` volume](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir) attached to the restoration job, which means that the binary logs are kept in the node storage where the job has been scheduled. This may not be suitable for large binary logs, as it can lead to exhausting the node's storage, resulting the restoration process to fail and potentially impacting other workloads running in the same node.

You are able to configure an alternative staging area using the `stagingStorage` field under the `bootstrapFrom` section in the `MariaDB` resource:


```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  bootstrapFrom:
    stagingStorage:
      persistentVolumeClaim:
        storageClassName: my-storage-class
        resources:
          requests:
            storage: 10Gi
        accessModes:
          - ReadWriteOnce
```

This will provision a PVC and attach it to the restoration job to be used as staging area.

## Limitations

- A `PointInTimeRecovery` object can only be referred by a single `MariaDB` object via the `pointInTimeRecoveryRef` field.
- A combination object storage bucket + prefix can only be utilizied by a single `MariaDB` instance to archive binary logs.

## Troubleshooting

The operator tracks the current archival status under the `MariaDB` status subresource. This status is updated after each archival cycle, and it contains metadata about the binary logs that have been archived, along with other useful information for troubleshooting:

```bash
kubectl get mariadb mariadb-repl -o jsonpath='{.status.pointInTimeRecovery}' | jq
{
  "lastArchivedBinaryLog": "mariadb-repl-bin.000001",
  "lastArchivedPosition": 358,
  "lastArchivedTime": "2026-03-02T11:14:00Z",
  "serverId": 10,
  "storageReadyForArchival": true
}
```

Additionally, also under the status subresource, the operator sets status conditions whenever a specific state of the binlog archival or point-in-time restoration process is reached:

```bash
kubectl get mariadb mariadb-repl -o jsonpath="{.status.conditions}" | jq
[
  {
    "lastTransitionTime": "2026-03-02T11:33:58Z",
    "message": "Archived binlogs",
    "reason": "ArchiveBinlogs",
    "status": "True",
    "type": "BinlogsArchived"
  },
  {
    "lastTransitionTime": "2026-03-01T12:16:40Z",
    "message": "Replayed binlogs",
    "reason": "ReplayBinlogs",
    "status": "True",
    "type": "BinlogsReplayed"
  },
]
```

The operator also emits Kubernetes events during both archival and restoration process, to either report an outstanding event or error:

```bash
kubectl get events --field-selector involvedObject.name=mariadb-repl --sort-by='.lastTimestamp'

24m         Warning   BinlogArchivalError    mariadb/mariadb-repl               Error archiving binary logs: 1 error occurred:...
23m         Normal    BinlogArchived         mariadb/mariadb-repl               Binary log mariadb-repl-bin.000001 archived
41s         Warning   MariaDBInitError       mariadb/mariadb-repl       Unable to init MariaDB: target recovery time 2026-02-28 21:10:42 +0100 CET is after latest recoverable time 2026-02-27 20:10:42 +0000 UTC
12s         Warning   BinlogTimelineInvalid   mariadb/mariadb-repl      Invalid binary log timeline: error getting binlog timeline between GTID 0-10-4 and target time 2026-02-28T21:10:42+01:00: timeline did not reach target time: 2026-02-28T21:10:42+01:00, last recoverable time: 2026-02-27T21:10:42+01:00
``` 

#### Common errors

##### Unable to start archival process

The following error will be returned if the archival process is configured pointing to a non-empty object storage, as the operator expects to start from a clean state:

```bash
kubectl get mariadb mariadb-repl -o jsonpath="{.status}" | jq
{
  "conditions": [
    {
      "lastTransitionTime": "2026-03-02T11:14:58Z",
      "message": "Error archiving binlogs: 1 error occurred:\n\t* binary log storage is not ready for archival. Archival must start from a clean state\n\n",
      "reason": "ArchiveBinlogsError",
      "status": "False",
      "type": "Ready"
    },
    {
      "lastTransitionTime": "2026-03-02T11:14:58Z",
      "message": "Error archiving binlogs: 1 error occurred:\n\t* binary log storage is not ready for archival. Archival must start from a clean state\n\n",
      "reason": "ArchiveBinlogsError",
      "status": "False",
      "type": "BinlogsArchived"
    }
  ],
}
``` 

To solve this, you can update the `PointInTimeRecovery` configuration pointing to another object storage bucket or prefix that is empty:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PointInTimeRecovery
metadata:
  name: pitr
spec:
  physicalBackupRef:
    name: physicalbackup-daily
  storage:
    s3:
      bucket: binlogs
      prefix: mariadb-v2 # previously it was "mariadb"
      endpoint: minio.minio.svc.cluster.local:9000
      region: us-east-1
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
```
After updating the `PointInTimeRecovery` configuration, the error will be cleared in the next archival cycle, and a new archival operation will be attempted.

Alternatively, you can also consider deleting the existing binary logs and [`index.yaml` inventory file](#binlog-inventory), only after having double checked that they are not needed for recovery.

##### Target recovery time is after latest recoverable time

This error is returned in the `MariaDB` init process, when the `targetRecoveryTime` provided to bootstrap is later than the [last recoverable time](#binlog-timeline-and-last-recoverable-time) reported by the `PointInTimeRecovery` status.

For example, if you have configured the `bootstrapFrom.targetRecoveryTime` field with the value `2026-02-28T20:10:42Z`, the following error will be returned:

```bash
kubectl get pitr
NAME   PHYSICAL BACKUP        LAST RECOVERABLE TIME   STRICT MODE   AGE
pitr   physicalbackup-daily   2026-02-27T20:10:42Z    true          43h

kubectl get mariadb
NAME           READY   STATUS                                                                                                                          PRIMARY          UPDATES                    AGE
mariadb-repl   False   Init error: target recovery time 2026-02-28 21:10:42 +0100 CET is after latest recoverable time 2026-02-27 20:10:42 +0000 UTC   mariadb-repl-0   ReplicasFirstPrimaryLast   65s
```

There are two ways to solve this issue:
- Update the `targetRecoveryTime` in the `MariaDB` resource to be earlier than or equal to the last recoverable time, which in this case is `2026-02-27T20:10:42Z`.
- Disable `strictMode` in the `PointInTimeRecovery` configuration, allowing to restore up until the latest recoverable time, in this case `2026-02-27T20:10:42Z`.

##### Invalid binary log timeline: error getting binlog timeline between GTID and target time: timeline did not reach target time

This error is returned when computing the binary log timeline during the restoration process, and it means that the operator could not build a timeline that reaches the `targetRecoveryTime` provided in the `bootstrapFrom` field of the `MariaDB` resource.

For example, if you have the following [binary log inventory](#binlog-inventory):

```yaml
apiVersion: v1
binlogs:
  server-10:
  ...
  - binlogFilename: mariadb-repl-bin.000003
    binlogVersion: 4
    firstGtid: 0-10-527
    firstTime: "2026-02-27T16:03:22Z"
    lastGtid: 0-10-1041
    lastTime: "2026-02-27T16:03:50Z"
    logPosition: 268493636
    previousGtids:
    - 0-10-526
    rotateEvent: true
    serverId: 10
    serverVersion: 11.8.5-2-MariaDB
    stopEvent: false
  - binlogFilename: mariadb-repl-bin.000004
    binlogVersion: 4
    firstGtid: 0-10-1042
    firstTime: "2026-02-27T16:03:50Z"
    lastGtid: 0-10-1559
    lastTime: "2026-02-27T16:04:15Z"
    logPosition: 268506819
    previousGtids:
    - 0-10-1041
    rotateEvent: true
    serverId: 10
    serverVersion: 11.8.5-2-MariaDB
    stopEvent: false
```

And your `targetRecoveryTime` is `2026-02-28T20:10:42Z`, the following error will be returned:

```bash
kubectl get mariadb
NAME           READY   STATUS                                                                                                                                                                                                                                                          PRIMARY          UPDATES                    AGE
mariadb-repl   False   Error replaying binlogs: Invalid binary log timeline: error getting binlog timeline between GTID 0-10-4 and target time 2026-02-28T21:10:42+01:00: timeline did not reach target time: 2026-02-28T21:10:42+01:00, last recoverable time: 2026-02-27T16:04:15Z   mariadb-repl-0   ReplicasFirstPrimaryLast   3m28s
```

There are two ways to solve this issue:
- Update the `targetRecoveryTime` in the `MariaDB` resource to be earlier than or equal to the last recoverable time, which in this case is `2026-02-27T16:04:15Z`.
- Disable `strictMode` in the `PointInTimeRecovery` configuration, allowing to restore up until the latest recoverable time, in this case `2026-02-27T16:04:15Z`.