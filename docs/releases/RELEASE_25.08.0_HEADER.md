**`{{ .ProjectName }}` [25.08.0](https://github.com/mariadb-operator/mariadb-operator/releases/tag/25.08.0) is here!** 🦭

We're thrilled to announce this new release fully loaded with exciting new features and bug fixes. This version is a significant step forward, enhancing the disaster recovery capabilities of the operator.

If you're upgrading from previous versions, don't miss the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_25.08.0.md)__ for a smooth transition.

### `PhysicalBackups` based on `mariadb-backup`

This release introduces the new `PhysicalBackup` CR for managing backups at the physical level. This MariaDB-native backup strategy relies on the `mariadb-backup` CLI, available in the official MariaDB container images. It enables a more efficient and faster backup process, especially for larger databases, compared to logical backups. 

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

Kudos to @harunkucuk5 for raising the [initial PR](https://github.com/mariadb-operator/mariadb-operator/pull/273)!

### `PhysicalBackups` based on `VolumeSnapshots`

As an alternative to `mariadb-backup`, you can also use [`VolumeSnapshots`](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) to create physical backups. This Kubernetes-native backup strategy relies on the CSI driver capabilities to create snapshots of the underlying data volumes at the storage level.

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

This will create a `VolumeSnapshot` of the data volume used by the `MariaDB` cluster. The `VolumeSnapshot` will be stored in the same namespace as the `PhysicalBackup` CR, and you can restore it by either referencing the parent `PhysicalBackup` in the `spec.bootstrapFrom` field of a new `MariaDB` cluster, or the `VolumeSnapshot` resource directly:

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

If you are using [LangChain](https://python.langchain.com/docs/introduction/) for building RAG applications, you may now leverage our new [MariaDB integration ](https://python.langchain.com/docs/integrations/vectorstores/mariadb/) to use MariaDB as vector store in LangChain.

### MariaDB cluster helm chart

We are introducing `mariadb-cluster`, a new helm chart that simplifies the deployment of a `MariaDB` cluster and its associated CRs managed by the operator. It allows you to manage all CRs in a single helm release, handling their relationships automatically so you don't need to configure the references manually.

Refer to the [helm documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/HELM.md) for further details.

Kudos to @hedgieinsocks for this initiative!

### Replication improvements

We have made some progress towards making our asynchronous replication feature GA. Refer to the PRs below for more details:
- [__BREAKING CHANGE__] Convert SyncBinlog to integer (https://github.com/mariadb-operator/mariadb-operator/pull/1324)
- Add optional delay for automatic failover (https://github.com/mariadb-operator/mariadb-operator/pull/1287)
- Do not trigger automatic failover when suspended (https://github.com/mariadb-operator/mariadb-operator/pull/1285)
- Fix replication probe (https://github.com/mariadb-operator/mariadb-operator/pull/1253)
- Add verbosity to skipped switchover steps (https://github.com/mariadb-operator/mariadb-operator/pull/1288)


Kudos to @hedgieinsocks for all these contributions!

### Fixes and enhancements

We have fixed a number of bugs and made several enhancements in this release. Refer to the changelog below for further detail. 

### New versioning scheme

We are adopting a new calendar-based versioning scheme, where the version number is based on the year and month of the release, followed by a patch number, similarly to the Ubuntu versioning scheme. 

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.