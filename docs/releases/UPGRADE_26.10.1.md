# 26.10.1 update guide

This guide illustrates, step by step, how to update to `26.10.1` from previous versions. This guide only applies if you are updating from a version prior to `26.10.x`, otherwise you may upgrade directly (see [Helm](../helm.md#updates)).

> [!TIP]
> The [OCI-based installation](../helm.md#oci-based-installation) is recommended. To migrate an existing release, run `helm upgrade --install` with the same release but pointing to the `oci://` path. Refer to [this guide](https://www.securecodebox.io/blog/2024/06/28/helm-chart-oci-registry-migration/) and [this example PR](https://github.com/mmontes11/k8s-infrastructure/pull/73) for further information. 

> [!CAUTION]
> When migrating `mariadb-operator-crds` to the [OCI-based installation](../helm.md#oci-based-installation), always use `helm upgrade` in-place. Running `helm uninstall` first will delete the CRDs and cascade-delete all CRs, causing downtime.

- The [data-plane](../data_plane.md) must be updated to the `26.10.1` version, as this release changes the `mariadb` container env (new `MARIADB_AUTO_UPGRADE` support via `updateStrategy.mariadbAutoUpgradeEnabled`). You must set `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources before updating the operator. Then, once updated, the operator will also be updating the data-plane based on its version:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
+   autoUpdateDataPlane: true
```

- At this point, you may proceed to update the operator:

Upgrade the `mariadb-operator-crds` helm chart to `26.10.1`:
```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds mariadb-operator/mariadb-operator-crds --version 26.10.1
```

Upgrade the `mariadb-operator` helm chart to `26.10.1`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 26.10.1
```

- Consider reverting `updateStrategy.autoUpdateDataPlane` back to `false` in your `MariaDB` object to avoid unexpected updates:

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

> [!IMPORTANT]
> __[MariaDB major version upgrades]__ To upgrade your `MariaDB` servers across a major version (e.g. `mariadb:11.8` to `mariadb:12.3.3`), set `updateStrategy.mariadbAutoUpgradeEnabled=true` **before** bumping `spec.image`, so that the official image entrypoint runs `mariadb-upgrade` on start and migrates the system schema when the `Pods` roll:
> ```diff
> spec:
> -  image: mariadb:11.8.6
> +  image: mariadb:12.3.3
>   updateStrategy:
> +  mariadbAutoUpgradeEnabled: true
> ```
> Without the migration, the `Pods` restart against an unmigrated datadir and every query touching `mysql.proc` fails. After the `Pods` have rolled successfully, you may set the flag back to `false`: `mariadb-upgrade` is a no-op when there is nothing to migrate.

> [!NOTE]
> The default `mariadb` image is `mariadb:12.3.3`. Existing `MariaDB` resources keep the image set in their spec, so updating the operator does not update your MariaDB servers: explicitly set `spec.image=mariadb:12.3.3` in the `MariaDB` CR to upgrade them.
