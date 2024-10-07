
`{{ .ProjectName }}` __[0.34.0](https://github.com/mariadb-operator/mariadb-operator/releases/tag/0.34.0)__ is out! 🦭

This release introduces more intuitive versioning, backup compression, and enhancements to Galera cluster recovery, along with several other new features. See the full details below.

To upgrade from older versions, be sure to follow the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPGRADE_0.34.0.md)__.

### Versioning

Many of you have reported that our previous versioning model was confusing, as we had different versions for the operator image, operator Helm chart and CRD Helm chart. See https://github.com/mariadb-operator/mariadb-operator/issues/891.

In this release, we're introducing a new versioning model where everything (operator image, Helm charts) uses the unified version `0.34.0`.

### Backup compression

You are able to compress backups by providing the compression algorithm you want to use in the  `spec.compression` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  mariaDbRef:
    name: mariadb
  compression: gzip
```

Currently the following compression algorithms are supported:
- `bzip2`: Good compression ratio, but slower compression/decompression speed compared to gzip.
- `gzip`: Good compression/decompression speed, but worse compression ratio compared to bzip2.
- `none`: No compression.

See https://github.com/mariadb-operator/mariadb-operator/pull/836.

Kudos to @vixns for this contribution! 🙏🏻

### Galera cluster recovery

We're continuously refining our Galera recovery process based on the issues you report!

Some of you have encountered situations where the recovery `Jobs` get stuck with the following error:

```bash
[ERROR] mariadbd: Can't lock aria control file '/var/lib/mysql/aria_log_control' for exclusive use, error: 11. Will retry for 30 seconds
```

This occurs because the `MariaDB` `Pods` create exclusive locks on the same PVCs that the `Jobs` try to mount. To resolve this, the operator now downscales the `StatefulSet` before initiating the recovery `Jobs`. See https://github.com/mariadb-operator/mariadb-operator/pull/904.

Another less frequent error is that, after not being able to bootstrap the cluster on the first attempt, the `MariaDB` `Pods` return the following error:

```bash
 [ERROR] WSREP: It may not be safe to bootstrap the cluster from this node. It was not the last one to leave the cluster and may not contain all the updates.
```

This can occur if a different `Pod` was selected to bootstrap the cluster during a previous attempt, leaving the previous `Pod` with the bootstrap configuration. To handle this, the operator now cleans up the bootstrap config on non-bootstrapping `Pods`. See https://github.com/mariadb-operator/mariadb-operator/pull/910

### Run operator in HA

See https://github.com/mariadb-operator/mariadb-operator/pull/899.

Kudos to @sennerholm for this contribution! 🙏🏻

### Extensibility

See https://github.com/mariadb-operator/mariadb-operator/pull/908 and https://github.com/mariadb-operator/mariadb-operator/pull/912.

Kudos to @hedgieinsocks for these contributions! 🙏🏻

### `Pod` role labels

See https://github.com/mariadb-operator/mariadb-operator/pull/909.

Kudos to @nocturo for this contribution! 🙏🏻

### Mutable `maxUserConnections`

See https://github.com/mariadb-operator/mariadb-operator/pull/918.

Kudos to @hedgieinsocks for this contribution! 🙏🏻

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.