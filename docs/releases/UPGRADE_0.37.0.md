# Upgrade guide 0.37.0

This guide illustrates, step by step, how to migrate to 0.37.0 from previous versions. We have introduced support for __TLS__ in this release, and it is __enabled and enforced by default__. Please follow the steps to avoid any issues.

> [!WARNING]
> Do not attempt to skip intermediate version upgrades. Upgrade progressively through each version.

For example, if upgrading from `0.0.33` to `0.37.0`:
An attempt to upgrade from `0.0.33` directly to `0.37.0` will result in unpredictable behavior.
An attempt to upgrade from `0.0.33` to `0.34.0`, then `0.35.0`, and then `0.37.0` will result in success.


> [!CAUTION]
> TLS is enabled and enforced by default, not following the migration steps may result in unexpected behavior.

> [!CAUTION]
> With the introduction of TLS, breaking changes in the Galera data-plane have been introduced. Make sure you follow the migration steps to upgrade without any issues.


### Migration steps

- Uninstall you current `mariadb-operator` for preventing conflicts:
```bash
helm uninstall mariadb-operator
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

- If you are planning to use TLS, and you are currently using Galera, please set the following options to enable it:
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
By setting these options, the operator will issue and configure certificates for `MariaDB`, but TLS will not be enforced in the connections i.e. both TLS and non TLS connections will be accepted. TLS enforcement will be optionally configured at the end of the migration process.

- If you are planning to use TLS, and you are currently using `MaxScale`, it is important to note that, unlike `MariaDB`, it does not support TLS and non-TLS connections simultaneously (see [limitations](../TLS.md#limitations)). For this reason, you must temporarily point your applications to `MariaDB` during the migration process. You can achieve this by configuring your application to use the [`MariaDB Services`](../HA.md#kubernetes-services). At the end of the `MariaDB` migration process, the `MaxScale` instance will need to be recreated in order to use TLS, and then you will be able to point your application back to `MaxScale`. Ensure all applications are pointing to `MariaDB` before moving on to the next step.

-  Upgrade `mariadb-operator` to `0.37.0`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 0.37.0 
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
  updateStrategy:
+   autoUpdateDataPlane: false
-   autoUpdateDataPlane: true
```

- `MariaDB` is now accepting TLS connections. Next, you need to [migrate your applications to use TLS](../TLS.md#secure-application-connections-with-tls) by pointing them to connect to `MariaDB` securely. Ensure all application connections are using TLS before moving on to the next step.
- For enhanced security, it is recommended to enforce TLS in all `MariaDB` connections by setting:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  tls:
+   required: true
```
This will trigger a rolling upgrade, make sure it finishes successfully before proceeding with the next step.

- If you are using `MaxScale`, now that the `MariaDB` migration is completed, you should follow these steos ti recreate your `MaxScale` instance with TLS:

Delete your previous `MaxScale` instance. It is very important that you wait until your old `MaxScale` instance is fully terminated to make sure that the old configuration is cleaned up by the operator:
```bash
kubectl delete mxs maxscale-galera
```
Create a new `MaxScale` instance with `tls.enabled=true`:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale
spec:
+ tls:
+   enabled: false
```

- `MaxScale` is now accepting TLS connections. Next, you need to [migrate your applications to use TLS](../TLS.md#secure-application-connections-with-tls) by pointing them to connect to `MaxScale` securely. If you have done this previously for `MariaDB`, you just need to update your application configuration to use the [`MaxScale Service`](../MAXSCALE.md#kubernetes-services) and its CA bundle.