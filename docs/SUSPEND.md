# Suspend

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.30

## Table of contents
<!-- toc -->
- [Suspended state](#suspended-state)
- [Suspend a resource](#suspend-a-resource)
<!-- /toc -->

## Suspended state

When a resource is suspended, all operations performed by the operator are disabled, including but not limited to:
- Provisioning
- Upgrades
- Volume resize
- Galera cluster recovery

More specifically, the reconciliation loop of the operator is omitted, anything part of it will not happen while the resource is suspended. This could be useful in __maintenance__ scenarios, where manual operations need to be performed, as it helps prevent conflicts with the operator.

## Suspend a resource

Currently, only `MariaDB` and `MaxScale` resources support suspension. You can enable it by setting `suspend=true`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  suspend: true
```

This results in the reconciliation loop being disabled and the status being marked as `Suspended`:

```bash
kubectl get mariadbs
NAME             READY   STATUS      PRIMARY POD        AGE
mariadb-galera   False   Suspended   mariadb-galera-0   25h
```

To re-enable it, simply remove the `suspend` setting or set it to `suspend=false`.