**`{{ .ProjectName }}` [25.10.3](https://github.com/mariadb-operator/mariadb-operator/releases/tag/25.10.3) is here!** 🦭

The focus of this release has been improving our backup and restore capabilities, along with various bug fixes and enhancements.

We are also announcing [support for Kubernetes 1.35](https://github.com/mariadb-operator/mariadb-operator/pull/1542/files) and sharing our roadmap for upcoming releases.

## `PhysicalBackup` target policy

You are now able to define a `target` for `PhysicalBackup` resources, allowing you to control in which `Pod` the backups will be scheduled:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup
spec:
  mariaDbRef:
    name: mariadb
  target: Replica
```

By default, the `Replica` policy is used, meaning that backups will only be scheduled on ready replicas. Alternatively, you can use the `PreferReplica` policy to schedule backups on replicas when available, or fall back to the primary if no replicas are available.

This is particularly useful in scenarios where you have a limited number of replicas, for instance, a primary-replica topology (single primary, single replica). By using the `PreferReplica` policy in this scenario, not only you ensure that backups are taken even if there are no available replicas, but also enables replica recovery operations, as they rely on `PhysicalBackup` resources successfully completing:

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
    size: 10Gi
  replicas: 2
  replication:
    enabled: true
    replica:
      bootstrapFrom:
        physicalBackupTemplateRef:
          name: physicalbackup-tpl
      recovery:
        enabled: true
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup-tpl
spec:
  mariaDbRef:
    name: mariadb-repl
    waitForIt: false
  schedule:
    suspend: true
  target: PreferReplica
  storage:
    persistentVolumeClaim:
      resources:
        requests:
          storage: 100Gi
      accessModes:
        - ReadWriteOnce
```

In the example above, a `MariaDB` primary-replica cluster is defined with the ability to recover and rebuild the replica from a `PhysicalBackup` taken on the primary, thanks to the `PreferReplica` target policy. 

For additional details, please refer to the [PhysicalBackup documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/physical_backup.md) and the [replica recovery section](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/replication.md#replica-recovery).

## Backup encryption

Logical and physical backups i.e. `Backup` and `PhysicalBackup` resources have gained support for encrypting backups on the server-side when using S3 storage. For doing so, you need to generate an encryption key and configure the backup resource to use it:

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: ssec-key
stringData:
  # 32-byte key encoded in base64 (use: openssl rand -base64 32)
  customer-key: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
---
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
      ssec:
        customerKeySecretKeyRef:
          name: ssec-key
          key: customer-key
```
In order to boostrap a new instance from an encrypted backup, you need to provide the same encryption key in the `MariaDB` `bootstrapFrom` section.

Kudos to @xavierleune for this initiative and the PR contributing this feature! 🎉

## Deprecating embedded `MaxScale`

To improve maintainability, minimize complexity and reduce the size of the CRD bundle (getting close to the [1MB hard limit](https://azure.github.io/azure-service-operator/design/adr-2023-02-helm-chart-size-limitations/#:~:text=Helm%20v3%20stores%20chart%20state,a%20maximum%20size%20of%201MB.)), we are deprecating the [`MaxScale` embedded definition inside the `MariaDB` CR](https://github.com/mariadb-operator/mariadb-operator/blob/v25.10.2/docs/maxscale.md#maxscale-embedded-in-mariadb) in favor of deploying `MaxScale` as a separate CR.

To make the transition easier, we are providing you with this [migration script](https://raw.githubusercontent.com/mariadb-operator/mariadb-operator/main/hack/migrate_maxscale_to_resource.sh). Refer to the [MaxScale documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/maxscale.md) for additional details.

## Roadmap

We are excited to share the roadmap for the upcoming releases:
- [ ] [Point In Time Recovery (PITR)](https://github.com/mariadb-operator/mariadb-operator/issues/507)
- [ ] [Multi-cluster topology](https://github.com/mariadb-operator/mariadb-operator/issues/1543)

---

## Community

Contributions of any kind are always welcome: adding yourself to the [list of adopters](https://github.com/mariadb-operator/mariadb-operator/blob/main/ADOPTERS.md), reporting issues, submitting pull requests, or simply starring the project! 🌟

## Enterprise

For enterprise users, see the __[MariaDB Enterprise Operator](https://mariadb.com/products/enterprise/kubernetes-operator/)__, a commercially supported Kubernetes operator from MariaDB with additional enterprise-grade features.