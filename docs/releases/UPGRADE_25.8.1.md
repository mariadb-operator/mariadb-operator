# Upgrade guide 25.08.1

This guide illustrates, step by step, how to migrate from previous versions. 

> [!NOTE]  
> Do not attempt to skip intermediate version upgrades. Upgrade progressively through each version.

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

- Upgrade `mariadb-operator-crds` to `25.8.1`:

```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds  mariadb-operator/mariadb-operator-crds --version 25.8.1
```

- If you are using Galera, you must update the data-plane to the `25.8.1` version (see https://github.com/mariadb-operator/mariadb-operator/pull/1266). 


If you want the operator to automatically update the data-plane (i.e. init and agent containers), you can set `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources:
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
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.38.1
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:25.8.1
    initContainer:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.38.1
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:25.8.1
```

- If you are using replication, and you have the `spec.replication.syncBinlog` field set, some breaking changes have been introduced that affect you (see https://github.com/mariadb-operator/mariadb-operator/pull/1324):

Please perform the following migrations on the `spec.replication.syncBinlog` field:
- If you previously set `syncBinlog=true`, set  `syncBinlog=1`
- If you previously set `syncBinlog=false`, set  `syncBinlog=0`
- If you want the binary log to be flushed to disk after a given `<number>` of events, set `syncBinlog=<number>`

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  replication:
-    syncBinlog: true
+    syncBinlog: 1      
```


-  Upgrade `mariadb-operator` to `25.8.1`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 25.8.1 
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