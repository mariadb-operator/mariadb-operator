
`{{ .ProjectName }}` __[0.34.0](https://github.com/mariadb-operator/mariadb-operator/releases/tag/0.34.0)__ is out! 🦭

This release introduces more intuitive versioning, backup compression, and enhancements to Galera cluster recovery, along with several other new features. See the full details below.

To upgrade from older versions, be sure to follow the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_0.34.0.md)__.

### Versioning

Many of you have reported that our previous versioning model was confusing, as we had different versions for the operator image, operator Helm chart and CRD Helm chart. See https://github.com/mariadb-operator/mariadb-operator/issues/891.

In this release, we're introducing a new versioning model where everything (operator image, Helm charts) uses the unified version `0.34.0`.

### Backup compression

You can now compress backups by specifying the desired compression algorithm in the new `compression` field:

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

We have extended the operator Helm chart to provide you with everything needed to run the operator in HA:
- Multiple replicas
- Configure `Pod` anti-affinity
- Configure `PodDisruptionBudgets`

You can achieve this by providing the following values to the helm chart:

```yaml
ha:
  enabled: true
  replicas: 3

affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: app.kubernetes.io/name
          operator: In
          values:
          - mariadb-operator
        - key: app.kubernetes.io/instance
          operator: In
          values:
          - mariadb-operator
      topologyKey: kubernetes.io/hostname

pdb:
  enabled: true
  maxUnavailable: 1
```

See https://github.com/mariadb-operator/mariadb-operator/pull/899.

Kudos to @sennerholm for this contribution! 🙏🏻

### `Pod` role labels

A new label `k8s.mariadb.com/role` is now added to the `MariaDB` `Pods`:

```bash
❯ kubectl get mariadbs
NAME             READY   STATUS    PRIMARY            UPDATES                    AGE
mariadb-galera   True    Running   mariadb-galera-0   ReplicasFirstPrimaryLast   79m

❯ kubectl get pods -l k8s.mariadb.com/role=primary
NAME               READY   STATUS    RESTARTS   AGE
mariadb-galera-0   2/2     Running   0          79m

❯ kubectl get pods -l k8s.mariadb.com/role=replica
NAME               READY   STATUS    RESTARTS   AGE
mariadb-galera-1   2/2     Running   0          79m
mariadb-galera-2   2/2     Running   0          79m
```
See https://github.com/mariadb-operator/mariadb-operator/pull/909.

Kudos to @nocturo for this contribution! 🙏🏻

### Mutable `maxUserConnections`

You may update the `maxUserConnections` field without having to recreate the `User` resource.

See https://github.com/mariadb-operator/mariadb-operator/pull/918.

Kudos to @hedgieinsocks for this contribution! 🙏🏻

### Extensibility

We have introduced several extensibility improvements for deploying `MariaDB`:
- Support for extra `Service` ports. See https://github.com/mariadb-operator/mariadb-operator/pull/912
- Support for named `initContainers` and `sidecarContainers`. See https://github.com/mariadb-operator/mariadb-operator/pull/908.

Kudos to @hedgieinsocks for these contributions! 🙏🏻


---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.