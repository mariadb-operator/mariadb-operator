# Physical backups

## Table of contents

<!-- toc -->
- [What is a physical backup?](#what-is-a-physical-backup)
- [Backup strategies](#backup-strategies)
- [Storage types](#storage-types)
- [Scheduling](#scheduling)
- [Compression](#compression)
- [Retention policy](#retention-policy)
- [Restoration](#restoration)
- [Target recovery time](#target-recovery-time)
- [Timeout](#timeout)
- [Extra options](#extra-options)
- [S3 credentials](#s3-credentials)
- [Staging area](#staging-area)
- [VolumeSnapshots](#volumesnapshots)
- [Important considerations and limitations](#important-considerations-and-limitations)
- [Troubleshooting](#troubleshooting)
<!-- /toc -->

## What is a physical backup?

A physical backup is a snapshot of the entire data directory (`/var/lib/mysql`), including all data files. This type of backup captures the exact state of the database at a specific point in time, allowing for quick restoration in case of data loss or corruption.

Physical backups are the recommended method for backing up `MariaDB` databases, especially in production environments, as they are faster and more efficient than [logical backups](./logical_backup.md).

## Backup strategies

Multiple strategies are available for performing physical backups, including:
- **mariadb-backup**: Taken using the [mariadb-backup](https://mariadb.com/docs/server/server-usage/backup-and-restore/mariadb-backup/full-backup-and-restore-with-mariadb-backup) utility, which is available in the `MariaDB` images. The operator supports scheduling `Jobs` to perform backups using this utility.
- **Kubernetes VolumeSnapshot**: Leverage [Kubernetes VolumeSnapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/)  to create snapshots of the persistent volumes used by the `MariaDB` `Pods`. This method relies on a compatible CSI (Container Storage Interface) driver that supports volume snapshots. See the [VolumeSnapshots](#volumesnapshots) section for more details.

In order to use `VolumeSnapshots`, you will need to provide a `VolumeSnapshotClass` that is compatible with your storage provider. The operator will use this class to create snapshots of the persistent volumes:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  storage:
    volumeSnapshot:
      volumeSnapshotClassName: csi-hostpath-snapclass
```

For the rest of compatible [backup storage types](#storage-types), the `mariadb-backup` CLI will be used to perform the backup. For instance, to use `S3` as backup storage:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  storage:
    s3:
      bucket: physicalbackups
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
```

## Storage types

Multiple storage types are supported for storing physical backups, including:
- **S3 compatible storage**: Store backups in a S3 compatible storage, such as [AWS S3](https://aws.amazon.com/s3/) or [Minio](https://github.com/minio/minio).
- **Persistent Volume Claims (PVC)**: Use any of the [StorageClasses](https://kubernetes.io/docs/concepts/storage/storage-classes/) available in your Kubernetes cluster to create a `PersistentVolumeClaim` (PVC) for storing backups.
- **Kubernetes Volumes**: Store backups in any of the [in-tree storage providers](https://kubernetes.io/docs/concepts/storage/volumes/#volume-types) supported by Kubernetes out of the box, such as NFS.
- **Kubernetes VolumeSnapshots**: Use [Kubernetes VolumeSnapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) to create snapshots of the persistent volumes used by the `MariaDB` `Pods`. This method relies on a compatible CSI (Container Storage Interface) driver that supports volume snapshots. See the [VolumeSnapshots](#volume-snapshots) section for more details.


## Scheduling

Physical backups can be scheduled using the `spec.schedule` field in the `PhysicalBackup` resource. The schedule is defined using a [Cron format](https://en.wikipedia.org/wiki/Cron) and allows you to specify how often backups should be taken:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  schedule:
    cron: "*/1 * * * *"
    suspend: false
    immediate: true
```

If you want to immediatly trigger a backup after creating the `PhysicalBackup` resource, you can set the `immediate` field to `true`. This will create a backup immediately, regardless of the schedule.

If you want to suspend the schedule, you can set the `suspend` field to `true`. This will prevent any new backups from being created until the `PhysicalBackup` is resumed.

## Compression

When using physical backups based on `mariadb-backup`, you are able to choose the compression algorithm used to compress the backup files. The available options are:
- `bzip2`: Good compression ratio, but slower compression/decompression speed compared to gzip.
- `gzip`: Good compression/decompression speed, but worse compression ratio compared to bzip2.
- `none`: No compression.

To specify the compression algorithm, you can use the `compression` field in the `PhysicalBackup` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  compression: bzip2
```

`compression` is defaulted to `none` by the operator.

## Retention policy

You can define a retention policy both for backups based on `mariadb-backup` and for `VolumeSnapshots`. The retention policy allows you to specify how long backups should be retained before they are automatically deleted. This can be defined via the `maxRetention` field in the `PhysicalBackup` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  maxRetention: 720h # 30 days
```

When using physical backups based on `mariadb-backup`, the operator will automatically delete backups files in the specified storage older than the retention period.

When using `VolumeSnapshots`, the operator will automatically delete the `VolumeSnapshot` resources older than the retention period using the Kubernetes API.

## Restoration

Physical backups can only be restored in brand new `MariaDB` instances without any existing data. This means that you cannot restore a physical backup into an existing `MariaDB` instance that already has data.

To perform a restoration, you can specify a `PhysicalBackup` as restoration source under the `spec.bootstrapFrom` field in the `MariaDB` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  bootstrapFrom:
    backupRef:
      name: physicalbackup
      kind: PhysicalBackup
```

This will take into account the backup strategy and storage type used in the `PhysicalBackup`, and it will perform the restoration accordingly.

As an alternative, you can also provide a reference to an S3 bucket that was previously used to store the physical backup files:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  bootstrapFrom:
    s3:
      bucket: physicalbackups
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
    backupContentType: Physical
```

It is important to note that the `backupContentType` field must be set to `Physical` when restoring from a physical backup. This ensures that the operator uses the correct restoration method.

To restore a `VolumeSnapshot`, you can provide a reference to a specific `VolumeSnapshot` resource in the `spec.bootstrapFrom` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  bootstrapFrom:
    volumeSnapshotRef:
      name: physicalbackup-20250611163352
```

## Target recovery time

By default, the operator will match the closest backup available to the current time. You can specify a different target recovery time by using the `targetRecoveryTime` field in the `PhysicalBackup` resource. This lets you define the exact point in time you want to restore to:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  bootstrapFrom:
    targetRecoveryTime: 2025-06-17T08:07:00Z
``` 

## Timeout

By default, both backups based on `mariadb-backup` and `VolumeSnapshots` will have a timeout of 1 hour. You can change this timeout by using the `timeout` field in the `PhysicalBackup` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  timeout: 2h
```

When timed out, the operator will delete the `Jobs` or `VolumeSnapshots` resources associated wit the `PhysicalBackup` resource. The operator will create new `Jobs` or `VolumeSnapshots` to retry the backup operation if the `PhysicalBackup` resource is still scheduled.

## Extra options

When taking backups based on `mariadb-backup`, you can specify extra options to be passed to the `mariadb-backup` command using the `args` field in the `PhysicalBackup` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  args:
    - "--verbose"
```

Refer to the [mariadb-backup documentation](https://mariadb.com/docs/server/server-usage/backup-and-restore/mariadb-backup/mariadb-backup-options) for a list of available options.

## S3 credentials

Credentials for accessing an S3 compatible storage can be provided via the `s3` key in the `storage` field of the `PhysicalBackup` resource. The credentials can be provided as a reference to a Kubernetes `Secret`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  storage:
    s3:
      bucket: physicalbackups
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
```

Alternatively, if you are running in EKS, you can use dynamic credentials from an EKS Service Account using EKS Pod Identity or IRSA:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mariadb-backup
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::<<account_id>>:role/my-role-irsa
```

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  serviceAccountName: mariadb-backup
  storage:
    s3:
      bucket: physicalbackups
      prefix: mariadb
      endpoint: s3.us-east-1.amazonaws.com
      region:  us-east-1
      tls:
        enabled: true
```

By leaving out the `accessKeyIdSecretKeyRef` and `secretAccessKeySecretKeyRef` credentials and pointing to the correct `serviceAccountName`, the backup `Job` will use the dynamic credentials from EKS.

## Staging area

> [!NOTE]  
> S3 backups based on `mariadb-backup` are the only scenario that requires a staging area.

When using S3 storage for backups, a staging area is used for keeping the external backups while they are being processed. By default, this staging area is an `emptyDir` volume, which means that the backups are temporarily stored in the node's local storage where the `PhysicalBackup` `Job` is scheduled. In production environments, large backups may lead to issues if the node doesn't have sufficient space, potentially causing the backup/restore process to fail.

Additionally, when restoring these backups, the operator will pull the backup files from S3, uncompress them if needded, and restore them to each of the `MariaDB` `Pods` in the cluster individually. To save network bandwidth and compute resources, a staging area is used to keep the uncompressed backup files after they have been restored to the first `MariaDB` `Pod`. This allows the operator to restore the same backup to the rest of `MariaDB` `Pods` seamlessly, without needing to pull and uncompress the backup again.

To configure the staging area, you can use the `stagingStorage` field in the `PhysicalBackup` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  storage:
    s3:
      bucket: physicalbackups
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
  stagingStorage:
    persistentVolumeClaim:
      resources:
        requests:
          storage: 1Gi
      accessModes:
        - ReadWriteOnce
```

Similarly, you may also use a staging area when [bootstrapping from backup](#restoration), in the `MariaDB` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  mariaDbRef:
    name: mariadb
  bootstrapFrom:
    s3:
      bucket: physicalbackups
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
    backupContentType: Physical
    stagingStorage:
      persistentVolumeClaim:
        resources:
          requests:
            storage: 1Gi
        accessModes:
          - ReadWriteOnce
```

In the examples above, a PVC with the default `StorageClass` will be provisioned to be used as staging area.

## `VolumeSnapshots`

> [!IMPORTANT]
> Before using this feature, ensure that you meet the following prerequisites :
> - [external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter) and its CRs are installed in the cluster.
> - You have a compatible CSI driver that supports `VolumeSnapshots` installed in the cluster.
> - You have a `VolumeSnapshotClass` configured configured for your CSI driver.

The operator is capable of creating [`VolumeSnapshot` resources](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) of the PVCs used by the `MariaDB` `Pods`. This allows you to create point-in-time snapshots of your data in a Kubernetes-native way, leveraging the capabilities of your storage provider.

Most of the fields described in this documentation apply to `VolumeSnapshots`, including scheduling, retention policy, and compression. The main difference with the `mariadb-backup` based backups is that the operator will not create a `Job` to perform the backup, but instead it will create a `VolumeSnapshot` resource directly.

In order to create consistent, point-in-time snapshots of the `MariaDB` data, the operator will perform the following steps:
1. Temporarily pause the `MariaDB` writes by executing a `FLUSH TABLES WITH READ LOCK` command in one of the secondary `Pods`.
2. Create a `VolumeSnapshot` resource of the data PVC mounted by the `MariaDB` primary `Pod`.
3. Wait until the `VolumeSnapshot` resources becomes ready. When timing out, the operator will delete the `VolumeSnapshot` resource and retry the operation.
4. Issue a `UNLOCK TABLE` statement.

## Important considerations and limitations

### Root credentials

When restoring a backup, the root credentials specified through the `spec.rootPasswordSecretKeyRef` field in the `MariaDB` resource must match the ones in the backup. These credentials are utilized by the liveness and readiness probes, and if they are invalid, the probes will fail, causing your `MariaDB` `Pods` to restart after the backup restoration.

### Restore `Job`

When using backups based on `mariadb-backup`, restoring and uncompressing large backups can consume significant compute resources and may cause restoration `Jobs` to become stuck due to insufficient resources. To prevent this, you can define the compute resources allocated to the `Job`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  bootstrapFrom:
    restoreJob:
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          memory: 1Gi
```

### `ReadWriteOncePod` access mode partially supported

When using backups based on `mariadb-backup`, the data PVC used by the `MariaDB` `Pod` cannot use the [`ReadWriteOncePod`](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes) access mode, as it needs to be mounted at the same time by both the `MariaDB` `Pod` and the `PhysicalBackup` `Job`. In this case, please use either the `ReadWriteOnce` or `ReadWriteMany` access modes instead.

Alternatively, if you want to keep using the `ReadWriteOncePod` access mode, you must use backups based on `VolumeSnapshots`, which do not require creating a `Job` to perform the backup and therefore avoid the volume sharing limitation.


### `PhysicalBackup` `Jobs` scheduling

`PhysicalBackup` `Jobs` must mount the data PVC used by the primary `MariaDB` `Pod`. To avoid scheduling issues caused by the commonly used `ReadWriteOnce` access mode, the operator schedules backup `Jobs` on the same node as `MariaDB` by default.

If you prefer to disable this behavior and allow `Jobs` to run on any node, you can set `podAffinity=false`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  podAffinity: false
```

This configuration may be suitable when using the `ReadWriteMany` access mode, which allows multiple `Pods` across different nodes to mount the volume simultaneously.

## Troubleshooting

Custom columns are used to display the status of the `PhysicalBackup` resource:

```bash
kubectl get physicalbackups

NAME             COMPLETE   STATUS    MARIADB   LAST SCHEDULED   AGE
physicalbackup   True       Success   mariadb   17s              17s
```

To get a higher level of detail, you can also check the `status` field directly:

```bash
kubectl get physicalbackups physicalbackup -o json | jq -r '.status'

{
  "conditions": [
    {
      "lastTransitionTime": "2025-07-14T07:01:14Z",
      "message": "Success",
      "reason": "JobComplete",
      "status": "True",
      "type": "Complete"
    }
  ],
  "lastScheduleCheckTime": "2025-07-14T07:00:00Z",
  "lastScheduleTime": "2025-07-14T07:00:00Z",
  "nextScheduleTime": "2025-07-15T07:00:00Z"
}
```

You may also check the related events for the `PhysicalBackup` resource to see if there are any issues:

```bash
kubectl get events --field-selector involvedObject.name=physicalbackup

LAST SEEN   TYPE     REASON                  OBJECT                                 MESSAGE
116s        Normal   WaitForFirstConsumer    persistentvolumeclaim/physicalbackup   waiting for first consumer to be created before binding
116s        Normal   JobScheduled            physicalbackup/physicalbackup          Job physicalbackup-20250714140837 scheduled
116s        Normal   ExternalProvisioning    persistentvolumeclaim/physicalbackup   Waiting for a volume to be created either by the external provisioner 'rancher.io/local-path' or manually by the system administrator. If volume creation is delayed, please verify that the provisioner is running and correctly registered.
116s        Normal   Provisioning            persistentvolumeclaim/physicalbackup   External provisioner is provisioning volume for claim "default/physicalbackup"
113s        Normal   ProvisioningSucceeded   persistentvolumeclaim/physicalbackup   Successfully provisioned volume pvc-7b7c71f9-ea7e-4950-b612-2d41d7ab35b7
```

### Common errors

#### `mariadb-backup` log copy incomplete: consider increasing `innodb_log_file_size`

In some situations, when using the `mariadb-backup` strategy, you may encounter the following error in the backup `Job` logs:

```bash
mariadb [00] 2025-08-04 09:15:57 Was only able to copy log from 58087 to 59916, not 68968; try increasing
innodb_log_file_size
mariadb mariabackup: Stopping log copying thread.[00] 2025-08-04 09:15:57 Retrying read of log at LSN=59916
```

This can be addressed by increasing the `innodb_log_file_size` in the `MariaDB` configuration. You can do this by adding the following to your `MariaDB` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
...
  myCnf: |
    [mariadb]
    innodb_log_file_size=200M
```

Refer to [MDEV-36159](https://jira.mariadb.org/browse/MDEV-36237) for further details on this issue.