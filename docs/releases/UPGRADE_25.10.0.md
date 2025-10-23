# Upgrade guide 25.10.0

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

- __[replication]__ ⚠️ __If you are using replication, you must perform a migration__. Make sure the `MariaDB` to migrate is in ready state and get a copy of its manifest:
```bash
kubectl get mariadb mariadb-repl -o yaml > mariadb-repl.yaml
```
- __[replication]__ Download and setup the migration script for replication:

```bash
wget -q "https://raw.githubusercontent.com/mariadb-operator/mariadb-operator/main/hack/migrate_v25.10.0.sh"
chmod +x migrate_v25.10.0.sh
```

- __[replication]__ Execute the migration script:
```bash
./migrate_v25.10.0.sh mariadb-repl.yaml
```

- __[replication]__ [`gtid_strict_mode`](https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#gtid_strict_mode) is now enabled by default. You may consider disabling it by setting `spec.replication.gtidStrictMode=false` in `migrated.mariadb-repl.yaml`

- __[replication]__  Replication user (`repl`) default credentials have been changed from `repl-password-$MARIADB` to  `$MARIADB-repl-password`, you may consider creating a `Secret` with your existing credentials. Otherwise the password will be rotated to a random one if you don't have a explicit reference in `spec.replication.replica.replPasswordSecretKeyRef`

- __[galera]__ If you are using Galera, replace `spec.galera.primary.automaticFailover` with `spec.galera.primary.autoFailover`. It is enabled by default, no action is needed if you are willing to rely on the default (`true`).

- Upgrade `mariadb-operator-crds` to `25.10.0`:

```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds  mariadb-operator/mariadb-operator-crds --version 25.10.0
```

- __[galera]__ ⚠️ __If you are using Galera, you must update the data-plane__ to the `25.10.0` version.

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
  name: mariadb-galera
spec:
  galera:
    agent:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.38.1
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:25.10.0
    initContainer:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.38.1
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:25.10.0
```

- __[replication]__ Apply the `v25.10.0` specification:
```bash
kubectl apply -f migrated.mariadb-repl.yaml
```

-  Upgrade `mariadb-operator` to `25.10.0`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 25.10.0 
```

- If you previously decided to downscale the operator, make sure you upscale it back:
```bash
kubectl scale deployment mariadb-operator --replicas=1
kubectl scale deployment mariadb-operator-webhook --replicas=1
```

- __[galera]__ If you previously set `updateStratety.autoUpdateDataPlane=true`, you may consider reverting the changes once the upgrades have finished:

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