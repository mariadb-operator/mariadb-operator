# 26.10.2 update guide

This guide illustrates, step by step, how to update to `26.10.2` from previous versions. This release has no breaking changes, only fixes: updating the [data-plane](../data_plane.md) is recommended but optional. If you are updating from a `26.10.x` version and prefer to skip the data-plane update, you may upgrade directly (see [Helm](../helm.md#updates)).

> [!TIP]
> The [OCI-based installation](../helm.md#oci-based-installation) is recommended. To migrate an existing release, run `helm upgrade --install` with the same release but pointing to the `oci://` path. Refer to [this guide](https://www.securecodebox.io/blog/2024/06/28/helm-chart-oci-registry-migration/) and [this example PR](https://github.com/mmontes11/k8s-infrastructure/pull/73) for further information. 

> [!CAUTION]
> When migrating `mariadb-operator-crds` to the [OCI-based installation](../helm.md#oci-based-installation), always use `helm upgrade` in-place. Running `helm uninstall` first will delete the CRDs and cascade-delete all CRs, causing downtime.

- Optionally, update the [data-plane](../data_plane.md) to the `26.10.2` version. This is recommended, as this release ships agent fixes: the replica readiness probe keeps using the last known `maxLagSeconds` when the Kubernetes API is unreachable, and the S3 and Azure Blob Storage clients used for binary log archival no longer require `tls.caSecretKeyRef` and no longer rewrite Amazon S3 endpoints to their dual-stack variant. However, it is not required: the operator works with the previous data-plane version, and skipping it saves a rolling update of your `MariaDB` `Pods` if you prefer stability over these fixes. To update it, set `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources before updating the operator. Then, once updated, the operator will also be updating the data-plane based on its version:
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

Upgrade the `mariadb-operator-crds` helm chart to `26.10.2`:
```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds mariadb-operator/mariadb-operator-crds --version 26.10.2
```

Upgrade the `mariadb-operator` helm chart to `26.10.2`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 26.10.2
```

- If you updated the data-plane, consider reverting `updateStrategy.autoUpdateDataPlane` back to `false` in your `MariaDB` object to avoid unexpected updates:

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
> The S3 client no longer rewrites Amazon S3 endpoints (e.g. `s3.<region>.amazonaws.com`) to their dual-stack variant, it now honors the configured endpoint. If your cluster relies on IPv6 to reach Amazon S3, explicitly set the dual-stack endpoint (e.g. `s3.dualstack.<region>.amazonaws.com`) in your `Backup`, `PhysicalBackup`, `PointInTimeRecovery` and `Restore` resources.

> [!NOTE]
> `spec.replication.agent.lifecycle` and `spec.galera.agent.lifecycle` were previously ignored and are now applied to the `agent` container. If you have set any of them, updating the operator will trigger a rolling update of your `MariaDB` `Pods`.

> [!NOTE]
> The way the operator tracks the MaxScale servers configuration has changed. After updating the operator, the servers of every `MaxScale` resource will be patched once via the MaxScale API, with no impact on the `Pods`.
