# Upgrade guide 0.37.0

This guide illustrates, step by step, how to migrate to 0.36.0 from previous versions. We have introduced support for __TLS__ in this release, and it is __enabled and enforced by default__. Please follow the steps to avoid any issues.

> [!WARNING]
> Do not attempt to skip intermediate version upgrades. Upgrade progressively through each version.

For example, if upgrading from `0.0.33` to `0.37.0`:
An attempt to upgrade from `0.0.33` directly to `0.37.0` will result in unpredictable behavior.
An attempt to upgrade from `0.0.33` to `0.34.0`, then `0.35.0`, and then `0.37.0` will result in success.


> [!CAUTION]
> TLS is enabled and enforced by default, not following the migration steps may result in unexpected behavior.

> [!CAUTION]
> With the introduction of TLS, breaking changes in the Galera data-plane have been introduced. Make sure you follow the migration steps to avoid any issues.


### Migration steps

- Uninstall you current `mariadb-operator` for preventing conflicts:
```bash
helm uninstall mariadb-operator
```
Alternatively, you may only downscale and delete the webhook configurations:
```bash
kubectl scale deployment mariadb-operator --replicas=0
kubectl scale deployment mariadb-operator-webhook --replicas=0
kubectl delete validatingwebhookconfiguration mariadb-operator-webhook
kubectl delete mutatingwebhookconfiguration mariadb-operator-webhook
```

- Upgrade `mariadb-operator-crds` to `0.37.0`:
```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds  mariadb-operator/mariadb-operator-crds --version 0.37.0
```

- The Galera data-plane must be updated, even if you are not planning to use TLS. By setting `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources, the operator will automatically update the data-plane for you as part of the rolling upgrade.
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
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.37.0
    initContainer:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.36.0
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.37.0
```

- If you rather not use TLS, you can safely disable it by setting:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
+  tls:
+    enabled: false
```
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale
spec:
+ tls:
+   enabled: false
```

- If you are planning to use TLS and are currently using Galera, please set the following options to enable it:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  tls:
+   enabled: true
+   required: false
+   galeraSSTEnabled: false
```
By setting these options, the operator will issue and configure certificates for `MariaDB`, but TLS will not be enforced in the connections i.e. both TLS and non TLS connections will be accepted.

- If you are planning to use TLS and you are currently using MaxScale, it is important to note that, unlike MariaDB, [it does not support TLS and non TLS connections simultaneously](https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#tlsssl-encryption). For this reason, it is recommended to temporarily point your applications to `MariaDB` during the migration. You can achieve this by configuring your application to use the [`MariaDB` `Services`](../HA.md#kubernetes-services). After doing so, you can safely enable TLS in `MaxScale` by setting:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  tls:
+   enabled: true
```

-  Upgrade `mariadb-operator` to `0.37.0`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 0.37.0 
```

Alternatively, if you previously decided to downscale the operator, make sure you upscale it back:
```bash
kubectl scale deployment mariadb-operator --replicas=1
kubectl scale deployment mariadb-operator-webhook --replicas=1
```

- Wait until the rolling upgrade has finished. If you are using the `OnDelete` or `Never` update strategies, you will need to manually delete the Pods to trigger the rolling upgrade. More information can be found in the [update strategies](../UPDATES.md) documentation.

- Optionally, if you are willing to enable SSL for the Galera SSTs, follow these steps in order:

First, run __[this migration script](../../hack/migrate_galera_sst_ssl.sh)__:
```bash
 ./hack/migrate_galera_sst_ssl.sh <mariadb-instance-name> # e.g. ./migrate_galera_sst_ssl.sh mariadb-galera
```

Then, set the following option to enable SSL for Galera SSTs:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  tls:
+   galeraSSTEnabled: true
```

This will trigger a rolling upgrade, make sure it finishes successfully before proceeding with the next step.

- If needed, set back `autoUpdateDataPlane=false` in `MariaDB` to avoid unexpected data-plane updates:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  galera:
    enabled: true

  updateStrategy:
+   autoUpdateDataPlane: false
-   autoUpdateDataPlane: true
```
- At this point, both `MariaDB` and `MaxScale` accept TLS connections. You may now migrate your applications to use TLS connections.
- If you are using `MaxScale`, you may point your applications back to the [`MaxScale` `Services`](../MAXSCALE.md#kubernetes-services).
- Finally, for enhanced security, it is recommended to enforce TLS in all `MariaDB` connections by setting:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  tls:
+   required: true
```