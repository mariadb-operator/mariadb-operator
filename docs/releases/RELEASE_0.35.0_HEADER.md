
`{{ .ProjectName }}` __[0.35.0](https://github.com/mariadb-operator/mariadb-operator/releases/tag/0.35.0)__ is out! 🦭

To upgrade from older versions, be sure to follow the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_0.35.0.md)__.

### Staging storage for S3 backups

When using S3 storage for backups, a staging area is used for keeping the external backups while they are being processed. By default, this staging area is an `emptyDir` volume, which means that the backups are temporarily stored in the node's local storage where the `Backup`/`Restore` `Job` is scheduled. In production environments, large backups may lead to issues if the node doesn't have sufficient space, potentially causing the backup/restore process to fail.

To overcome this limitation, you are now able to define your own staging area by setting the `stagingStorage` field to both the `Backup` and `Restore` CRs:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  storage:
    s3:
      ...
  stagingStorage:
    persistentVolumeClaim:
      resources:
        requests:
          storage: 10Gi
      accessModes:
        - ReadWriteOnce
```

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Restore
metadata:
  name: restore
spec:
  s3:
    ...
  stagingStorage:
    persistentVolumeClaim:
      resources:
        requests:
          storage: 10Gi
      accessModes:
        - ReadWriteOnce
```

In the examples above, a PVC with the default `StorageClass` will be used as staging area. Refer to the [API reference](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/API_REFERENCE.md) for more configuration options.

### More flexibility configuring Kubernetes types

We have recently slimmed down our CRDs, resulting in a [~81% size decrease](https://github.com/mariadb-operator/mariadb-operator/pull/869). As part of this massive refactor, we have replaced the upstream Kubernetes types and introduce our custom types. In this release, we keep committed to this matter, and we have extended our Kubernetes types to ensure flexibility, including:
- `nodeAffinity` as expression-driven alternative to `nodeSelector`
- `configMap` and `secret` volume sources support
- `env` support for both `initContainers` and `sidecarContainers`
- `resources` support in metrics exporter `Deployment`

Refer to the [API reference](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/API_REFERENCE.md) for more details about this fields.

Kudos to @am6737 for helping with this! 🙏🏻

### Enhanced session affinity for MaxScale GUI

In previous releases, the MaxScale GUI `Service` used `sessionAffinity` to avoid load balancing, ensuring that GUI requests stayed with the same Pod. This was important because each MaxScale `Pod` operates as an independent server, maintaining its own user sessions for the GUI.

When using an API gateway in front of the MaxScale GUI `Service` without `sessionAffinity` configured, users may experience unexpected logouts, as sessions from one server are invalid on another. To address this, we now point the MaxScale GUI `Service` to a specific `Pod`, dynamically updating the target if the selected `Pod` goes down. This approach ensures consistency and predictability, minimizing the chances of sending GUI requests to new MaxScale `Pods` whenever possible. See https://github.com/mariadb-operator/mariadb-operator/pull/956.

Refer to the [MaxScale docs](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/MAXSCALE.md#maxscale-gui) for further detail.

### Support for image digests in Helm chart

You can now specify image digests when installing the operator Helm chart. Instead of providing a `tag`, you will need to specify a `digest` under the image values:

```yaml
image:
  repository: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator
  pullPolicy: IfNotPresent
  digest: sha256:084a927ee9f3918a5c85d283f73822ae205757df352218de0b935853a0765060

webhook:
  enabled: true
  image:
    repository: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator
    pullPolicy: IfNotPresent
    digest: sha256:084a927ee9f3918a5c85d283f73822ae205757df352218de0b935853a0765060

certController:
  enabled: true
  image:
    repository: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator
    pullPolicy: IfNotPresent
    digest: sha256:084a927ee9f3918a5c85d283f73822ae205757df352218de0b935853a0765060
```

Kudos to @am6737 for this contribution! 🙏🏻

### Replication improvements

During an update, make sure that at least one `Pod` has replication configured before proceeding with the update of the primary. See https://github.com/mariadb-operator/mariadb-operator/pull/947.

Kudos to @BonySmoke for this contribution! 🙏🏻

### Various fixes

See https://github.com/mariadb-operator/mariadb-operator/pull/932 https://github.com/mariadb-operator/mariadb-operator/pull/924.

Kudos to @am6737 for this contributions! 🙏🏻

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.