# Upgrade guide 0.37.1

This guide illustrates, step by step, how to migrate to `0.37.1` from previous versions.

> [!WARNING]
> Avoid skipping intermediate version upgrades. Always upgrade progressively, one version at a time, and follow the upgrade guide step by step.

> [!CAUTION]
> With the introduction of TLS, breaking changes in the Galera data-plane have been introduced. Make sure you follow the migration steps to upgrade without any issues.


### Migration steps

1. Uninstall you current `mariadb-operator` for preventing conflicts:
```bash
helm uninstall mariadb-operator
```

2. Upgrade `mariadb-operator-crds` to `0.37.1`:
```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds  mariadb-operator/mariadb-operator-crds --version 0.37.1
```

3. The Galera data-plane must be updated, even if you are not planning to use TLS. By setting `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources, the operator will automatically update the data-plane for you as part of the rolling upgrade.
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
+   autoUpdateDataPlane: true
```

Alternatively, you can also do this manually:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  galera:
    agent:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.36.0
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.37.1
    initContainer:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.36.0
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.37.1
```

4. Upgrade `mariadb-operator` to `0.37.1`. This will trigger a rolling upgrade, make sure it finishes successfully before proceeding with the next step. Refer to the [updates documentation](../UPDATES.md) for further information about update strategies:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 0.37.1 
```

5. If needed, set back `autoUpdateDataPlane=false` in `MariaDB` to avoid unexpected data-plane updates in the future:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
+   autoUpdateDataPlane: false
-   autoUpdateDataPlane: true
```

6. If you plan to use TLS, please refer to the __[TLS documentation](../TLS.md)__. 