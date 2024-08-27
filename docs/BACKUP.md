# Backup and Restore

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.28

`mariadb-operator` allows you to declarativaly take backups by defining `Backup` resources and later on restore them by using their `Restore` counterpart. These resources get reconciled into `Job`/`CronJob` resources that automatically perform the backup/restore operations, so you don't need to manually script them.

## Table of contents
<!-- toc -->
- [Storage types](#storage-types)
- [`Backup` CR](#backup-cr)
- [`Restore` CR](#restore-cr)
- [Bootstrap new `MariaDB` instances](#bootstrap-new-mariadb-instances)
- [Backup and restore specific databases](#backup-and-restore-specific-databases)
- [Extra options](#extra-options)
- [Important considerations and limitations](#important-considerations-and-limitations)
- [Logical backups](#logical-backups)
- [Migrating an external MariaDB to a `MariaDB` running in Kubernetes](#migrating-an-external-mariadb-to-a-mariadb-running-in-kubernetes)
- [Migrating to a `MariaDB` with different topology](#migrating-to-a-mariadb-with-different-topology)
- [Minio reference installation](#minio-reference-installation)
- [Reference](#reference)
- [Troubleshooting](#troubleshooting)
<!-- /toc -->

## Storage types

Currently, the following storage types are supported:
- **[S3](../examples/manifests/backup_s3.yaml) compatible storage**: Store backups in a S3 compatible storage, such as [AWS S3](https://aws.amazon.com/s3/) or [Minio](https://github.com/minio/minio). 
- **[PVCs](../examples/manifests/backup.yaml)**: Use the available [StorageClasses](https://kubernetes.io/docs/concepts/storage/storage-classes/) in your Kubernetes cluster to provision a PVC dedicated to store the backup files.
- **[Kubernetes volumes](../examples/manifests/backup_nfs.yaml)**: Use any of the [volume types](https://kubernetes.io/docs/concepts/storage/volumes/#volume-types) supported natively by Kubernetes.

Our recommendation is to store the backups externally in a [S3](../examples/manifests/backup.yaml) compatible storage. [Minio](https://github.com/minio/minio) makes this incredibly easy, take a look at our [Minio reference installation](#minio-reference-installation) to quickly spin up an instance.

## `Backup` CR

You can take a one-time backup of your `MariaDB` instance by declaring the following resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  mariaDbRef:
    name: mariadb
  storage:
    persistentVolumeClaim:
      resources:
        requests:
          storage: 100Mi
      accessModes:
        - ReadWriteOnce
```
This will use the default `StorageClass` to provision a PVC that would hold the backup files, but ideally you should use a S3 compatible storage:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  mariaDbRef:
    name: mariadb
  storage:
    s3:
      bucket: backups
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
```
By providing the authentication details and the TLS configuration via references to `Secret` keys, this example will store the backups in a local Minio instance.

#### Scheduling

To minimize the Recovery Point Objective (RPO) and mitigate the risk of data loss, it is recommended to perform backups regularly. You can do so by providing a `spec.schedule` in your `Backup` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup-scheduled
spec:
  mariaDbRef:
    name: mariadb
  backupRef:
    name: backup
  schedule:
    cron: "*/1 * * * *"
    suspend: false
```

This resource gets reconciled into a `CronJob` that periodically takes the backups.

It is important to note that regularly scheduled `Backups` complement very well the [target recovery time](#target-recovery-time) feature detailed below.

#### Retention policy

Given that the backups can consume a substantial amount of storage, it is crucial to define your retention policy by providing the `spec.maxRetention` field in your `Backup` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup-scheduled
spec:
  mariaDbRef:
    name: mariadb
  backupRef:
    name: backup
  maxRetention: 720h # 30 days
```

By default, it will be set to `720h` (30 days), indicating that backups older than 30 days will be automatically deleted.

## `Restore` CR

You can easily restore a `Backup` in your `MariaDB` instance by creating the following resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Restore
metadata:
  name: restore
spec:
  mariaDbRef:
    name: mariadb
  backupRef:
    name: backup
```

This will trigger a `Job` that will mount the same storage as the `Backup` and apply the dump to your `MariaDB` database.

Nevertheless, the `Restore` resource doesn't necessarily need to specify a `spec.backupRef`, you can point to other storage source that contains backup files, for example a S3 bucket:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Restore
metadata:
  name: restore
spec:
  mariaDbRef:
    name: mariadb
  s3:
    bucket: backups
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
```

#### Target recovery time

If you have multiple backups available, specially after configuring a [scheduled Backup](#scheduling), the operator is able to infer which backup to restore based on the `spec.targetRecoveryTime` field.

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Restore
metadata:
  name: restore
spec:
  mariaDbRef:
    name: mariadb
  backupRef:
    name: backup
  targetRecoveryTime: 2023-12-19T09:00:00Z
```

The operator will look for the closest backup available and utilize it to restore your `MariaDB` instance.

By default, `spec.targetRecoveryTime` will be set to the current time, which means that the latest available backup will be used.

## Bootstrap new `MariaDB` instances

To minimize your Recovery Time Objective (RTO) and to switfly spin up new clusters from existing `Backups`, you can provide a `Restore` source directly in the `MariaDB` object via the `spec.bootstrapFrom` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-from-backup
spec:
  storage:
    size: 1Gi
  bootstrapFrom:
    backupRef:
      name: backup
    targetRecoveryTime: 2023-12-19T09:00:00Z
```

As in the `Restore` resource, you don't strictly need to specify a reference to a `Backup`, you can provide other storage types that contain backup files:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-from-backup
spec:
  storage:
    size: 1Gi
  bootstrapFrom:
    s3:
      bucket: backups
      prefix: mariadb
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
    targetRecoveryTime: 2023-12-19T09:00:00Z
```

Under the hood, the operator creates a `Restore` object just after the `MariaDB` resource becomes ready. The advantage of using `spec.bootstrapFrom` over a standalone `Restore` is that the `MariaDB` is bootstrap-aware and this will allow the operator to hold primary switchover/failover operations until the restoration is finished.

## Backup and restore specific databases

By default, all the logical databases are backed up when a `Backup` is created, but you may also select specific databases by providing the `databases` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  mariaDbRef:
    name: mariadb
  databases:
    - db1
    - db2
    - db3
```

When it comes to restore, all the databases available in the backup will be restored, but you may also choose a single database to be restored via the  `database` field available in the `Restore` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Restore
metadata:
  name: restore
spec:
  mariaDbRef:
    name: mariadb
  backupRef:
    name: backup
  databases: db1
```

There are a couple of points to consider here:
- The referred database (`db1` in the example) must previously exist for the `Restore` to succeed.
- The `mariadb` CLI invoked by the operator under the hood only supports selecting a single database to restore via the [`--one-database`](https://mariadb.com/kb/en/mariadb-command-line-client/#-o-one-database) option, restoration of multiple specific databases is not supported.

## Extra options

Not all the flags supported by `mariadb-dump` and `mariadb` have their counterpart field in the `Backup` and `Restore` CRs respectively, but you may pass extra options by using the `args` field. For example, setting the `--verbose` flag can be helpful to track the progress of backup and restore operations:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  mariaDbRef:
    name: mariadb
  args:
    - --verbose
```
```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Restore
metadata:
  name: restore
spec:
  mariaDbRef:
    name: mariadb
  backupRef:
    name: backup
  args:
    - --verbose
```

Refer to the `mariadb-dump` and `mariadb` CLI options in the [reference](#reference) section.

## Important considerations and limitations

#### Root credentials

When restoring a backup, the root credentials specified through the `spec.rootPasswordSecretKeyRef` field in the `MariaDB` resource must match the ones in the backup. These credentials are utilized by the liveness and readiness probes, and if they are invalid, the probes will fail, causing your `MariaDB` `Pods` to restart after the backup restoration.

#### Restore job

Restoring large backups can consume significant compute resources and may cause `Restore` `Jobs` to become stuck due to insufficient resources. To prevent this, you can define the compute resources allocated to the `Job`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  storage:
    size: 1Gi
  bootstrapFrom:
    restoreJob:
      args:
        - --verbose
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          memory: 1Gi
``` 

#### Galera backup limitations

#### `mysql.global_priv`

Galera only replicates the tables with InnoDB engine:
- https://galeracluster.com/library/kb/user-changes.html

Something that does not include `mysql.global_priv`, the table used to store users and grants, which uses the MyISAM engine. This basically means that a Galera instance with `mysql.global_priv` populated will not replicate this data to an empty Galera instance. However, DDL statements (`CREATE USER`, `ALTER USER` ...) will be replicated.

Taking this into account, if we think now about a restore scenario where:
- The backup file includes a `DROP TABLE` statement for the `mysql.global_priv` table.
- The backup has some `INSERT` statements for the `mysql.global_priv` table.
- The Galera cluster has 3 nodes: `galera-0`, `galera-1` and `galera-2`.
- The backup is restored in `galera-0`.

This is what will happen under the scenes while restoring the backup:
- The `DROP TABLE` statement is a DDL so it will be executed in `galera-0`, `galera-1` and `galera-2`.
- The `INSERT` statements are not DDLs, so they will only be applied to `galera-0`.
- This results in the `galera-1` and `galera-2` not having the `mysql.global_priv` table.

After the backup is fully restored, the liveness and readiness probes will kick in, they will succeed in `galera-0`, but they will fail in `galera-1` and `galera-2`, as they rely in the root credentials available in `mysql.global_priv`, resulting in the `galera-1` and `galera-2` getting restarted.

To address this issue, when backing up `MariaDB` instances with Galera enabled, the `mysql.global_priv` table will be excluded from backups by using the `--ignore-table` option with `mariadb-dump`. This prevents the replication of the `DROP TABLE` statement for the `mysql.global_priv` table. You can opt-out from this feature by setting `spec.ignoreGlobalPriv=false` in the `Backup` resource.

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  mariaDbRef:
    name: mariadb
  ignoreGlobalPriv: false
```

Also, to avoid situations where `mysql.global_priv` is unreplicated, all the entries in that table must be managed via DDLs. This is the recommended approach suggested in the [Galera docs](https://galeracluster.com/library/kb/user-changes.html). There are a couple of ways that we can guarantee this:
- Use the `rootPasswordSecretKeyRef`, `username` and `passwordSecretKeyRef` fields of the `MariaDB` CR to create the root and initial user respectively. This fields will be translated into DDLs by the image entrypoint.
- Rely on the [`User`](https://github.com/mariadb-operator/mariadb-operator/blob/main/examples/manifests/user.yaml) and [`Grant`](https://github.com/mariadb-operator/mariadb-operator/blob/main/examples/manifests/grant.yaml) CRs to create additional users and grants. They will be translated into DDL statements (`CREATE USER`, `GRANT`) by the operator.


#### `LOCK TABLES` 

Galera is not compatible with the `LOCK TABLES` statement:
- https://mariadb.com/kb/en/lock-tables/#limitations

For this reason, the operator automatically adds the `--skip-add-locks` option to the `Backup` to overcome this limitation.

## Logical backups

Logical backups serve not just as a source of restoration, but also enable data mobility between `MariaDB` instances. These backups are called "logical" because they are independent from the `MariaDB` topology, as they only contain DDLs and `INSERT` statements to populate data.

As of today, `mariadb-operator` only supports logical backups, but we have plans to implement Point-In-Time-Recovery(PITR) based on physical backups and binary logs. This will allow to restore the state of a `MariaDB` instance in a particular point in time, minimizing the RPO (data loss) and RTO (time to recover). See https://github.com/mariadb-operator/mariadb-operator/issues/507.

## Migrating an external MariaDB to a `MariaDB` running in Kubernetes

You can leverage [logical backups](#logical-backups) to bring your external MariaDB data into a new `MariaDB` instance running in Kubernetes. Follow this runbook for doing so:

1. Take a logical backup of your external MariaDB using one of the commands below:
```bash
mariadb-dump --user=${MARIADB_USER} --password=${MARIADB_PASSWORD} --host=${MARIADB_HOST} --single-transaction --events --routines --all-databases > backup.2024-08-26T12:24:34Z.sql
```
> [!IMPORTANT]  
> If you are using Galera or planning to migrate to a Galera instance, make sure you understand the [Galera backup limitations](#galera-backup-limitations) and use the following command instead:

```bash
mariadb-dump --user=${MARIADB_USER} --password=${MARIADB_PASSWORD} --host=${MARIADB_HOST} --single-transaction --events --routines --all-databases --skip-add-locks --ignore-table=mysql.global_priv > backup.2024-08-26T12:24:34Z.sql
```

2. Ensure that your backup file is named in the following format: `backup.2024-08-26T12:24:34Z.sql`. If the file name does not follow this format, it will be ignored by the operator.

3. Upload the backup file to one of the supported [storage types](#storage-types). We recommend using S3.

4. Create your `MariaDB` resource declaring that you want to [bootstrap from the previous backup](#bootstrap-new-mariadb-instances) and providing a [root password `Secret`](#root-credentials) that matches the backup:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
  replicas: 3
  galera:
    enabled: true
  storage:
    size: 1Gi
  bootstrapFrom:
    s3:
      bucket: backups
      prefix: mariadb
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
    targetRecoveryTime: 2024-08-26T12:24:34Z
```

5. If you are using Galera in your new instance, migrate your previous users and grants to use the `User` and `Grant` CRs.

## Migrating to a `MariaDB` with different topology

Databa mobility between `MariaDB` instances with different topologies is possible with [logical backups](#logical-backups). However, there are a couple of technical details that you need to be aware of in the following scenarios:

#### Migrating between standalone and replicated `MariaDBs`

This should be fully compatible, no issues have been detected.

#### Migrating from standalone/replicated to Galera `MariaDBs`

There are a couple of limitations regarding the backups in Galera, please make sure you read the [Galera backup limitations](#galera-backup-limitations) section before proceeding.

To overcome this limitations, the `Backup` in the standalone/replicated instance needs to be taken with `spec.ignoreGlobalPriv=true`. In the following example, we are backing up a standalone `MariaDB` (single instance):

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup-standalone
spec:
  mariaDbRef:
    name: mariadb-standalone
  ignoreGlobalPriv: true
```

Once the previous `Backup` is completed, we will be able bootstrap a new Galera instance from it:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  replicas: 3
  galera:
    enabled: true
  storage:
    size: 1Gi
  bootstrapFrom:
    backupRef:
      name: backup-standalone
```

## Minio reference installation

The easiest way to get a S3 compatible storage is [Minio](https://github.com/minio/minio). You can install it by using their [helm chart](https://github.com/minio/minio/tree/master/helm/minio), or, if you are looking for a production-grade deployment, take a look at their [operator](https://github.com/minio/operator).

In our case, we have have [configured](../hack/config/minio.yaml) a Minio instance for [development](./DEVELOPMENT.md) purposes, you can easily install it by running:

```bash
make cluster
make install-minio
make net # to access the console via a MetalLB LoadBalancer: https://minio-console:9001
```
As an alternative, you can also use [play.min.io](https://play.min.io/) using these [credentials](../examples/manifests/config/minio-secret.yaml).

## Reference
- [API reference](./API_REFERENCE.md)
- [Example suite](../examples/)
- [`mariadb-dump` options](https://mariadb.com/kb/en/mariadb-dump/#options)
- [`mariadb` options](https://mariadb.com/kb/en/mariadb-command-line-client/#options)


## Troubleshooting

#### Galera `Pods` restarting after bootstrapping from a backup

Please make sure you understand the [Galera backup limitations](#galera-backup-limitations).

After doing so, ensure that your backup does not contain a `DROP TABLE mysql.global_priv;` statement, as it will make your liveness and readiness probes to fail after the backup restoration.
