# 26.10 update guide

This guide illustrates, step by step, how to update to `26.10.0` from previous versions. This guide only applies if you are updating from a version prior to `26.10.x`, otherwise you may upgrade directly (see [Helm](../helm.md#updates)).

> [!TIP]
> The [OCI-based installation](../helm.md#oci-based-installation) is recommended. To migrate an existing release, run `helm upgrade --install` with the same release but pointing to the `oci://` path. Refer to [this guide](https://www.securecodebox.io/blog/2024/06/28/helm-chart-oci-registry-migration/) and [this example PR](https://github.com/mmontes11/k8s-infrastructure/pull/73) for further information. 

> [!CAUTION]
> When migrating `mariadb-operator-crds` to the [OCI-based installation](../helm.md#oci-based-installation), always use `helm upgrade` in-place. Running `helm uninstall` first will delete the CRDs and cascade-delete all CRs, causing downtime.

- The [data-plane](../data_plane.md) must be updated to the `26.10.0` version, as this release changes the [replication](../replication.md) configuration rendered by the init container (semi-synchronous state per role) and the replication liveness probe served by the agent. You must set `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources before updating the operator. Then, once updated, the operator will also be updating the data-plane based on its version:
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

Upgrade the `mariadb-operator-crds` helm chart to `26.10.0`:
```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds mariadb-operator/mariadb-operator-crds --version 26.10.0
```

Upgrade the `mariadb-operator` helm chart to `26.10.0`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 26.10.0
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

> [!NOTE]
> __[replication]__ Once updated, the operator enforces `rpl_semi_sync_master_enabled` per role: enabled in the primary and disabled in the replicas. Previously it was enabled in every node. No action is required, this only applies when [semi-synchronous replication](../replication.md#asynchronous-vs-semi-synchronous-replication) is enabled (the default). Optionally, you may set `replication.semiSyncBootAsReplica=true` so nodes boot `read_only` and are never writable while unable to require a replica acknowledgement.

> [!NOTE]
> The default `mariadb` image is now `mariadb:12.3.3`. Existing `MariaDB` resources keep the image set in their spec, so updating the operator does not update your MariaDB servers: explicitely set `spec.image=mariadb:12.3.3` in the `MariaDB` CR to upgrade them.
