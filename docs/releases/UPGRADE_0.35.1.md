# Upgrade guide 0.35.1

This guide illustrates, step by step, how to migrate to `0.35.1` from previous versions. 

> [!NOTE]  
> Do not attempt to skip intermediate version upgrades. Upgrade progressively through each version.

For example, if upgrading from `0.0.31` to `0.0.33`:
An attempt to upgrade from `0.0.31` directly to `0.0.33` will result in will result in unpredictable behavior.
An attempt to upgrade from `0.0.31` to `0.0.32` and then `0.0.33` will result in success.

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

- Upgrade `mariadb-operator-crds` to `0.35.1`:

```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds  mariadb-operator/mariadb-operator-crds --version 0.35.1
```

- If you are using Galera, and you want the operator to automatically update the data-plane (i.e. init and agent containers) to `0.35.1`, you can set `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources:

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
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.35.0
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.35.1
    initContainer:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.35.0
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.35.1
```

-  Upgrade `mariadb-operator` to `0.35.1`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 0.35.1 
```

- If you previously decided to downscale the operator, make sure you upscale it back:
```bash
kubectl scale deployment mariadb-operator --replicas=1
kubectl scale deployment mariadb-operator-webhook --replicas=1
```

- If you previously set `updateStratety.autoUpdateDataPlane=true`, you may consider reverting the changes once the upgrades have finished:

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
