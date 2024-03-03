# Upgrade guide v0.0.26

> [!NOTE]  
> APIs are currently in `v1alpha1`, which implies that non backward compatible changes might happen. See [Kubernetes API versioning](https://kubernetes.io/docs/reference/using-api/#api-versioning) for more detail.

This guide illustrates, step by step, how to migrate to `v0.0.26` from previous versions, as some breaking changes have been introduced in the `MariaDB` resource. See the changes grouped by field:

`apiVersion`
https://github.com/mariadb-operator/mariadb-operator/pull/418

`storage`
https://github.com/mariadb-operator/mariadb-operator/pull/407

`serviceAccountName`
https://github.com/mariadb-operator/mariadb-operator/pull/416

`galera`
https://github.com/mariadb-operator/mariadb-operator/pull/384
https://github.com/mariadb-operator/mariadb-operator/pull/394

Follow these steps for upgrading:

- In your current `mariadb-operator` version, get a manifest of your `MariaDB` resource:
```bash
kubectl get mariadbs.mariadb.mmontes.io mariadb-galera -o yaml > mariadb-galera.yaml
```

- Download and setup the migration script:
```bash
wget -q "https://raw.githubusercontent.com/mariadb-operator/mariadb-operator/main/hack/migrate_v0.0.26.sh"
chmod +x migrate_v0.0.26.sh
```

- Install `v0.0.26` CRDs:
> [!NOTE]  
> Helm does not handle CRD upgrades. See [helm docs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations).

```bash
kubectl apply --server-side=true --force-conflicts -f https://github.com/mariadb-operator/mariadb-operator/releases/download/helm-chart-0.26.0/crds.yaml
```

- Make sure you `MariaDB` is in ready state and execute the migration script:
> [!IMPORTANT]  
> `MariaDB` must be in a ready state.

```bash
./migrate_v0.0.26.sh mariadb-galera.yaml
```

- Apply the `v0.0.26` specification:
```bash
kubectl apply -f migrated.mariadb-galera.yaml
```

- Patch the `v0.0.26` status:
```bash
kubectl patch mariadbs.k8s.mariadb.com mariadb-galera --subresource status --type merge -p "$(cat status.mariadb-galera.yaml)"
```

- Uninstall you current `mariadb-operator`:
```bash
helm uninstall mariadb-operator
```

- If your `MariaDB` has Galera enabled, delete the `mariadb-galera` `Role`, as it will be specyfing the old CRDs:
```bash
kubectl delete role mariadb-galera
```

- Install the current `mariadb-operator` version:
```bash
helm repo update mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator
```

- Cleanup old CRDs:
```bash
OLD_HELM_VERSION=0.25.0
kubectl delete -f https://github.com/mariadb-operator/mariadb-operator/releases/download/helm-chart-${OLD_HELM_VERSION}/crds.yaml
```