**`{{ .ProjectName }}` [25.08.0](https://github.com/mariadb-operator/mariadb-operator/releases/tag/25.08.0) is here!** 🦭

We're thrilled to announce this new release fully loaded with exciting new features and bug fixes. This version is a significant step forward, enhancing the disaster recovery capabilities of this operator.

If you're upgrading from previous versions, don't miss the [UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_25.08.0.md) for a smooth transition.

### `PhysicalBackups` based on `mariadb-backup`

This release introduces the new `PhysicalBackup` CR for managing backups at the physical level. The MariaDB-native strategy relies on the `mariadb-backup` CLI, available in our container images. This enables a more efficient and faster backup process, especially for larger databases, compared to logical backups. 

In order to use this, you can define a `PhysicalBackup` in your cluster:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
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

This CR allows you to schedule physical backups, manage its retention policy, compression and the storage backend to use. In order to restore a backup, you can spin up a new `MariaDB` cluster and reference the `PhysicalBackup` in the `spec.bootstrapFrom` field:

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
    targetRecoveryTime: 2025-06-17T08:07:00Z
```

This will fetch, uncompress and prepare the backup on each nodes of the cluster, in a more efficient and faster way than logical backups. For more details, check the [physical backups documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/physical_backup.md).

### `PhysicalBackups` based on `VolumeSnapshots`

As an alternative to `mariadb-backup`, you can also use [`VolumeSnapshots`](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) to create physical backups. This Kubernetes-native strategy relies on the CSI driver capabilities to create snapshots of the underlying data volumes at the storage level.

To use this method, you can define a `PhysicalBackup` in your cluster as follows:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  schedule:
    cron: "0 0 * * *"
    suspend: false
    immediate: true
  maxRetention: 720h
  storage:
    volumeSnapshot:
      volumeSnapshotClassName: csi-hostpath-snapclass
```

This will create a `VolumeSnapshot` object of the data volume used by the `MariaDB` cluster. The `VolumeSnapshot` will be stored in the same namespace as the `PhysicalBackup` CR, and you can restore it by either referencing the `PhysicalBackup` in the `spec.bootstrapFrom` field of a new `MariaDB` cluster, or the `VolumeSnapshot` directly:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-from-volumesnapshot
spec:
  storage:
    size: 10Gi
    storageClassName: csi-hostpath-sc
  bootstrapFrom:
    volumeSnapshotRef:
      name: physicalbackup-20250610165200 
``` 

For more details, check the [physical backups documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/physical_backup.md).

### MariaDB 11.8 and `VECTOR` support

MariaDB 11.8 is now supported and used as default version by this operator.

This version introduces the [`VECTOR` data type](https://mariadb.com/docs/server/reference/sql-structure/vectors/vector-overview), which allows you to store and operate with high-dimensional vectors natively in the database. This is particularly useful for AI applications, as they require to operate with vector embeddings.

If you are using [Langchain](https://python.langchain.com/docs/introduction/) for building RAG applications, you may now use our new [MariaDB integration ](https://python.langchain.com/docs/integrations/vectorstores/mariadb/) to use MariaDB as a vector store.

### MariaDB cluster helm chart



### Replication improvements

### Fixes and enhancements

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.