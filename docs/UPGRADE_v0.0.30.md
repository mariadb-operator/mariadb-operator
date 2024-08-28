# Upgrade guide v0.0.30

This guide illustrates, step by step, how to migrate to `v0.0.30` from previous versions. 

This release ships outstanding changes that make the Galera recovery process notably more robust. For making this possible, the `v0.0.30` operator relies some functionality available in both the `init` and `agent` containers used for Galera, so they both need to be updated to `v0.0.30` as detailed in further steps.

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

- Upgrade CRDs to `v0.0.30`:
> [!IMPORTANT]  
> Helm does not handle CRD upgrades. See [helm docs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations).

```bash
kubectl replace -f https://github.com/mariadb-operator/mariadb-operator/releases/download/helm-chart-0.30.0/crds.yaml
```

- If you are using Galera, apply the following changes in the `MariaDB` resources:

Update the `init` and `agent` containers to `v0.0.30`:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  galera:
    agent:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.29
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.30
    initContainer:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.29
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.30
```

`podSyncTimeout` and `podRecoveryTimeout` defaults have been bumped to `5m`, make sure you bump them as well to at least `5m`:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  galera:
    recovery:
-      podRecoveryTimeout: 3m
+      podRecoveryTimeout: 5m
-      podSyncTimeout: 3m
+      podSyncTimeout: 5m
```

`minClusterSize` defaults to `1` replica now, which means that the recovery process will only be triggered when all the `Pods` are down. This is the recommended setting now, as it fits better the new recovery process:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  galera:
    recovery:
-      minClusterSize: "50%"
+      minClusterSize: 1
```
 
-  Upgrade `mariadb-operator` to `v0.0.30`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 0.30.0 
```

- If you previously decided to downscale the operator, make sure you upscale it back:
```bash
kubectl scale deployment mariadb-operator -n default --replicas=1
kubectl scale deployment mariadb-operator-webhook -n default --replicas=1
```