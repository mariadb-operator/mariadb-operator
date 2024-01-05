# Backup and Restore

> [!WARNING]  
> This documentation applies to `mariadb-operator` version >= v0.0.24

`mariadb-operator` allows you to declarativaly take backups by defining `Backup` resources and later on restore them by using their `Restore` counterpart. These resources get reconciled into `Job`/`CronJob` resources that automatically perform the backup/restore operations, so you don't need to manually operate your `MariaDB`.

Refer to the sections below, the [API reference](./API_REFERENCE.md) and the [example suite](../examples/) to see see how to configure the `Backup` and `Restore` resources.
 
## Storage types

Currently, the following storage types are supported:
- **[S3](../examples/manifests/mariadb_v1alpha1_backup.yaml) compatible storage**: Store backupss in a S3 compatible storage, such as [AWS S3](https://aws.amazon.com/s3/) or [Minio](https://github.com/minio/minio). 
- **[PVCs](../examples/manifests/mariadb_v1alpha1_backup_pvc.yaml)**: Use the available [StorageClasses](https://kubernetes.io/docs/concepts/storage/storage-classes/) in your Kubernetes cluster to provision a PVC dedicated to store the backup files.
- **[Kubernetes volumes](../examples/manifests/mariadb_v1alpha1_backup_nfs.yaml)**: Use any of the [volume types](https://kubernetes.io/docs/concepts/storage/volumes/#volume-types) supported natively by Kubernetes.

Our recommendation is to store the backups externally in a [S3](../examples/manifests/mariadb_v1alpha1_backup.yaml) compatible storage. [Minio](https://github.com/minio/minio) makes this incredibly easy, take a look at our [Minio reference installation](#minio-reference-installation) to quickly spin up an instance.

## `Backup`

You can take a one-time backup of your `MariaDB` instance by declaring the following resource:

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
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
apiVersion: mariadb.mmontes.io/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  mariaDbRef:
    name: mariadb
  storage:
    s3:
      bucket: backups
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
apiVersion: mariadb.mmontes.io/v1alpha1
kind: Backup
metadata:
  name: backup-scheduled
spec:
  mariaDbRef:
    name: mariadb
  schedule:
    cron: "*/1 * * * *"
    suspend: false
...
```

This resource gets reconciled into a `CronJob` that periodically takes the backups.

It is important to note that regularly scheduled `Backups` complement very well the [target recovery time](#target-recovery-time) feature detailed below.

#### Retention policy

Given that the backups can consume a substantial amount of storage, it is crucial to define your retention policy by providing the `spec.maxRetention` field in your `Backup` resource:

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: Backup
metadata:
  name: backup-scheduled
spec:
  mariaDbRef:
    name: mariadb
  maxRetention: 720h # 30 days
...
```

By default, it will be set to `720h` (30 days), indicating that backups older than 30 days will be automatically deleted.

## `Restore`

You can easily restore a `Backup` in your `MariaDB` instance by creating the following resource:

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
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
apiVersion: mariadb.mmontes.io/v1alpha1
kind: Restore
metadata:
  name: restore
spec:
  mariaDbRef:
    name: mariadb
  s3:
    bucket: backups
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
apiVersion: mariadb.mmontes.io/v1alpha1
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

#### Bootstrap new `MariaDB` instances from `Backups`

To minimize your Recovery Time Objective (RTO) and to switfly spin up new clusters from existing `Backups`, you can provide a `Resource` source directly in the `MariaDB` object via the `spec.bootstrapFrom` field:

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
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
apiVersion: mariadb.mmontes.io/v1alpha1
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

Under the hood, the operator creates a `Restore` object just after the `MariaDB` resource becomes ready.

## Minio reference installation

The easiest way to get a S3 compatible storage is [Minio](https://github.com/minio/minio). You can install it by using their [helm chart](https://github.com/minio/minio/tree/master/helm/minio), or, if you are looking for a production-grade deployment, take a look at their [operator](https://github.com/minio/operator).

In our case, we have have [configured](../hack/config/minio.yaml) a Minio instance for [development](./DEVELOPMENT.md) purposes, you can easily install it by running:

```bash
make cluster
make install-minio
make net # to access the console via a MetalLB LoadBalancer: https://minio-console:9001
```