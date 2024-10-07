
`{{ .ProjectName }}` __[0.34.0](https://github.com/mariadb-operator/mariadb-operator/releases/tag/0.34.0)__ is out! 🦭

This release introduces more intuitive versioning, backup compression, and enhancements to Galera cluster recovery, along with several other new features. See the full details below.

To upgrade from older versions, be sure to follow the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPGRADE_0.34.0.md)__.

### Versioning

https://github.com/mariadb-operator/mariadb-operator/issues/891

### Backup compression

See: https://github.com/mariadb-operator/mariadb-operator/pull/836

Kudos to @vixns for this contribution! 🙏🏻

### Galera cluster recovery

```bash
2024-09-25 21:08:57 0 [ERROR] mariadbd: Can't lock aria control file '/var/lib/mysql/aria_log_control' for exclusive use, error: 11. Will retry for 30 seconds
```

See https://github.com/mariadb-operator/mariadb-operator/pull/904.

```bash
 [ERROR] WSREP: It may not be safe to bootstrap the cluster from this node. It was not the last one to leave the cluster and may not contain all the updates.
```

See https://github.com/mariadb-operator/mariadb-operator/pull/910


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