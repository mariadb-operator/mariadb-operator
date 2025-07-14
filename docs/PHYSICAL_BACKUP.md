# Physical backups

## Table of contents

## What is a physical backup?

A physical backup is a snapshot of the entire data directory (`/var/lib/mysql`), including all data files. This type of backup captures the exact state of the database at a specific point in time, allowing for quick restoration in case of data loss or corruption.

Physical backups are the recommended method for backing up `MariaDB` databases, especially in production environments, as they are faster and more efficient than [logical backups](./LOGICAL_BACKUP.md).

## Backup strategies

Multiple strategies are available for performing physical backups, including:
- **mariadb-backup**: Taken using the [mariadb-backup](https://mariadb.com/docs/server/server-usage/backup-and-restore/mariadb-backup/full-backup-and-restore-with-mariadb-backup) utility, which is part of the `MariaDB` server package. The operator supports scheduling `Jobs` to perform backups using this utility.
- **Kubernetes Volume Snapshot**: Leverage [Kubernetes Volume Snapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/)  to create snapshots of the persistent volumes used by the `MariaDB` pods. This method relies on a compatible CSI (Container Storage Interface) driver that supports volume snapshots. See the [Volume Snapshots](#volume-snapshots) section for more details.

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

For the rest of compatible backup storage types, the `mariadb-backup` CLI will be used to perform the backup. For instance, to use `S3` as backup storage:

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
- **Kubernetes Volumes**: Store backups in any of the [in-tree storage provider](https://kubernetes.io/docs/concepts/storage/volumes/) supported by Kubernetes out of the box, such as NFS.
- **Kubernetes Volume Snapshot**: Use [Kubernetes Volume Snapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) to create snapshots of the persistent volumes used by the `MariaDB` pods. This method relies on a compatible CSI (Container Storage Interface) driver that supports volume snapshots. See the [Volume Snapshots](#volume-snapshots) section for more details.


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

## Retention policy

## Restoration

From PhysicalBackup, S3 (with backupContentType), NFS and VolumeSnapshot

RestoreJob

## Target recovery time

## Timeout

## Extra options

## S3 credentials

## Staging area

## Volume Snapshots

## Important considerations and limitations

Root password. Restore job. ReadwriteOncePod not supported: https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes

## Troubleshooting

Custom columns are used to display the status of the `PhysicalBackup` resource:

```bash
kubectl get physicalbackups

NAME             COMPLETE   STATUS    MARIADB   LAST SCHEDULED   AGE
physicalbackup   True       Success   mariadb   17s              17s
```

To get a higher level of detail, you can also check the `status` field directly::

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