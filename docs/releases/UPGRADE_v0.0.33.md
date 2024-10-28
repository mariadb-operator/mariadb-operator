# Upgrade guide v0.0.33

This guide illustrates, step by step, how to migrate to `v0.0.33` from previous versions. 

> [!NOTE]  
> Do not attempt to skip intermediate version upgrades. Upgrade progressively through each version.

For example, if upgrading from `0.0.31` to `0.0.33`:
An attempt to upgrade from `0.0.31` directly to `0.0.33` will result in unpredictable behavior.
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

- Helm has certain [limitations when managing CRDs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations). Therefore, beginning with the current version, we will adopt [this approach recommended in the official Helm documentation](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#method-2-separate-charts), which involves managing CRDs in a separate chart. First, let's specify the release name and namespace for installing the new `mariadb-operator-crds` chart:

```bash
export CRDS_RELEASE_NAME="<HELM-RELEASE-NAME>" # e.g. mariadb-operator-crds
export CRDS_RELEASE_NAMESPACE="<HELM-RELEASE-NAMESPASE>" # e.g. databases
```

- If you installed previous versions of the `mariadb-operator` helm chart, you need to patch the CRDs to be owned by the new `mariadb-operator-crds` helm chart:

```bash
for crd in $(kubectl get crd -o jsonpath='{range .items[?(@.spec.group=="k8s.mariadb.com")]}{.metadata.name}{"\n"}{end}'); do
  kubectl annotate crd $crd \
    meta.helm.sh/release-name=$CRDS_RELEASE_NAME \
    meta.helm.sh/release-namespace=$CRDS_RELEASE_NAMESPACE --overwrite
  kubectl label crd $crd \
    app.kubernetes.io/managed-by=Helm --overwrite
done
```

- Install the new `mariadb-operator-crds` helm chart:

```bash
helm repo update mariadb-operator
helm install $CRDS_RELEASE_NAME -n $CRDS_RELEASE_NAMESPACE mariadb-operator/mariadb-operator-crds --version 0.0.33 
```

- If you are using Galera, and you want the operator to automatically update the data-plane (i.e. init and agent containers) to `v0.0.33`, you can set `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
+   autoUpdateDataPlane: true
```

If want to progressively update your fleet of databases, you may also set `updateStrategy.type=Never` in some of them:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
+   type: Never
```

-  Upgrade `mariadb-operator` to `v0.0.33`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 0.33.0 
```

- If you previously decided to downscale the operator, make sure you upscale it back:
```bash
kubectl scale deployment mariadb-operator -n default --replicas=1
kubectl scale deployment mariadb-operator-webhook -n default --replicas=1
```

- If you previously set `updateStratety.autoUpdateDataPlane=true` and/or `updateStratety.type=Never`, you may consider reverting the changes once the upgrades have finished:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
+   type: ReplicasFirstPrimaryLast
-   autoUpdateDataPlane: true
```
