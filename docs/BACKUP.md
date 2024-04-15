# Backup and Restore

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.28

`mariadb-operator` allows you to declarativaly take backups by defining `Backup` resources and later on restore them by using their `Restore` counterpart. These resources get reconciled into `Job`/`CronJob` resources that automatically perform the backup/restore operations, so you don't need to manually operate your `MariaDB`.

## Table of contents
<!-- toc -->
- [Storage types](#storage-types)
- [<code>Backup</code> CR](#backup-cr)
    - [Scheduling](#scheduling)
    - [Retention policy](#retention-policy)
- [<code>Restore</code> CR](#restore-cr)
    - [Target recovery time](#target-recovery-time)
- [Bootstrap new <code>MariaDB</code> instances](#bootstrap-new-mariadb-instances)
- [Backup and restore specific databases](#backup-and-restore-specific-databases)
- [Extra arguments](#extra-arguments)
- [Galera limitations](#galera-limitations)
- [Migrating to a <code>MariaDB</code> with different topology](#migrating-to-a-mariadb-with-different-topology)
- [Minio reference installation](#minio-reference-installation)
- [Reference](#reference)
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
  volumeClaimTemplate:
    resources:
      requests:
        storage: 1Gi
    accessModes:
      - ReadWriteOnce
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
  volumeClaimTemplate:
    resources:
      requests:
        storage: 1Gi
    accessModes:
      - ReadWriteOnce
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
- The `mariadb` CLI invoked by the operator under the hood ony supports the [`--one-database`](https://mariadb.com/kb/en/mariadb-command-line-client/#-o-one-database) flag, multiple databases are not supported.

## Extra arguments

Not all the flags supported by `mariadb-dump` and `mariadb` have their counterpart field in the `Backup` and `Restore` CRs respectively, but you may pass extra flags by using `args`. For instance, it may be useful to set the `--verbose` flag to see how the backup and restore makes progress:

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

## Galera limitations

Galera only replicates the tables with InnoDB engine:
- https://galeracluster.com/library/kb/user-changes.html

Something that does not include `mysql.global_priv`: the table used to store users and grants. This basically means that a Galera instance with `mysql.global_priv` populated will not replicate this data to an empty Galera B. This is something to take into account when defining your backup strategy.

By default, the SQL dumps generated by the `Backup` resource include `DROP TABLE` statements to make them idempotent. When they are restored in a `MariaDB` with Galera enabled, the `DROP TABLE` is propagated to the replicas, but not its data, resulting in authentication errors in the replicas. Thi is specially critical because the `livenessProbes` will fail with authentication errors, resulting in `Pods` restarting. More information can be found in the following issue:
- https://github.com/mariadb-operator/mariadb-operator/issues/556

To overcome this, `--ignore-table=mysql.global_priv` is added by default to the `Backup` resources pointing to a `MariaDB` with Galera enabled, so there is no action on your side to be done.

The downside of adding `--ignore-table=mysql.global_priv` is that the users and its grants have to be managed and recreated by either:
- MariaDB image entrypoint, by using the `spec.rootPasswordSecretKeyRef`, `spec.username` and `spec.passwordSecretKeyRef` fields.
- The `User` and `Grant` CRs, which will be eventually reconciled by the operator.

In any case, you still have control over this feature by using the `Backup`'s `spec.ignoreGlobalPriv`, which is defaulted to `true` when Galera is enabled and `false` otherwise:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  mariaDbRef:
    name: mariadb
  ignoreGlobalPriv: true
```

## Migrating to a `MariaDB` with different topology

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
