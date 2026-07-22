# 26.03 update guide

This guide illustrates, step by step, how to update to `26.3.0` from previous versions. This guide only applies if you are updating from a version prior to `26.3.x`, otherwise you may upgrade directly (see [Helm](../helm.md#updates))

- The [data-plane](../data_plane.md) must be updated to the `26.3.0` version. You must set `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources before updating the operator. Then, once updated, the operator will also be updating the data-plane based on its version:
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

Upgrade the `mariadb-operator-crds` helm chart to `26.3.0`:
```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds mariadb-operator/mariadb-operator-crds --version 26.3.0
```

Upgrade the `mariadb-operator` helm chart to `26.3.0`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 26.3.0
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