As we celebrate the 10th anniversary of the __[first Kubernetes commit](https://github.com/kubernetes/kubernetes/commit/2c4b3a562ce34cddc3f8218a2c4d11c7310e6d56)__ 🎉, we’re thrilled to announce the release of `{{ .ProjectName }}`🦭 __[v0.0.29](https://github.com/mariadb-operator/mariadb-operator/releases/tag/v0.0.29)__ ! A version fully loaded of new features, enhancements and bug fixes. Check out the detailed list below.

### New helm repository

We are migrating to a new helm repository: `helm.mariadb.com`. Please make sure you migrate, as the previous repository will be deprecated in further versions:

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator
```

### Rolling updates

This version brings a new role-aware update strategy: `ReplicasFirstPrimaryLast`. By using these strategy, the operator will roll out replica `Pods` first, waiting for each of them to become ready, and then proceed with the primary `Pod`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  updateStrategy:
    type: ReplicasFirstPrimaryLast
```

If you want to have full control on the update process, you may use the `OnDelete` strategy instead: `Pods` will not be updated until you manually delete them.

Refer to the [updates documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPDATES.md) for further detail.

### `my.cnf` configuration

`myCnf` and `myCnfConfigMapKeyRef` fields are now mutable. You may now tweak system variables without having to recreate your `MariaDB` resource, for example, you may update the `innodb_buffer_pool_size` for tuning your instances according to your performance needs:

```diff
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
-   innodb_buffer_pool_size=1024M
+   innodb_buffer_pool_size=2048M
    max_allowed_packet=256M 
```

Whenever `my.cnf` changes, to ensure your configuration changes are applied, the operator will automatically initiate a rolling update based on your predefined update strategy.

Refer to the [my.cnf documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/CONFIGURATION.md#mycnf) for further detail.

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.