# Upgrade guide v0.0.31

This guide illustrates, step by step, how to migrate to `v0.0.31` from previous versions. 

> [!NOTE]  
> Do not attempt to skip intermediate version upgrades. Upgrade progressively through each version.

For example, if upgrading from `0.0.28` to `0.0.31`:
An attempt to upgrade from `0.0.28` directly to `0.0.30` will result in unpredictable behavior.
An attempt to upgrade from `0.0.28` to `0.0.30` and then `0.0.31` will result in success.

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

- Upgrade CRDs to `v0.0.31`:
> [!IMPORTANT]  
> Helm does not handle CRD upgrades. See [helm docs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations).

```bash
kubectl replace -f https://github.com/mariadb-operator/mariadb-operator/releases/download/helm-chart-0.31.0/crds.yaml
```

- If you are using Galera, apply the following changes in the `MariaDB` resources:

Update the `init` and `agent` containers to `v0.0.31`:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  galera:
    agent:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.30
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.31
    initContainer:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.30
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.31
```

-  Upgrade `mariadb-operator` to `v0.0.31`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 0.31.0 
```

- If you previously decided to downscale the operator, make sure you upscale it back:
```bash
kubectl scale deployment mariadb-operator -n default --replicas=1
kubectl scale deployment mariadb-operator-webhook -n default --replicas=1
```
