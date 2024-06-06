Announcing the release of `{{ .ProjectName }}`🦭 __[v0.0.29](https://github.com/mariadb-operator/mariadb-operator/releases/tag/v0.0.29)__ ! A version fully loaded of new features, enhancements and bug fixes. Check out the detailed list below.

### New helm repository

We are migrating to a new helm repository: helm.mariadb.com. Please make sure you migrate, as the previous repository will be deprecated in further versions:

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator
```

### Rolling updates

This version brings a new role aware update strategy: `ReplicasFirstPrimaryLast`. By using these strategy, replica `Pods` will be rolled one by one first, and lastly the primary `Pods`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  updateStrategy:
    type: ReplicasFirstPrimaryLast
``` 

Refer to the [updates documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPDATES.md) for further detail.

### `my.cnf` configuration

`myCnf` and `myCnfConfigMapKeyRef` are now mutable and a rolling update is triggered to the `MariaDB` resource whenever they change. You may now tweak system variables without having to recreate your `MariaDB` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  myCnf: |
    [mariadb]
    bind-address=*
    default_storage_engine=InnoDB
    binlog_format=row
    innodb_autoinc_lock_mode=2
    innodb_buffer_pool_size=1024M
    max_allowed_packet=256M 
```

Refer to the [my.cnf documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/CONFIGURATION.md#mycnf) for further detail.

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.