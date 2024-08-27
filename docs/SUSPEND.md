# Suspend

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.30

By leveraging the automation provided by `mariadb-operator`, you can declaratively manage large fleets of databases using CRs. This also covers day two operations, such as upgrades, which can be risky when rolling out updates to thousands of instances simultaneously.

To mitigate this, and to give you more control on when these operations are performed, you are able to selectively suspend a subset of `MariaDB` resources, temporarily stopping the upgrades and other operations on them.

## Table of contents
<!-- toc -->
- [Suspended state](#suspended-state)
- [Suspend a resource](#suspend-a-resource)
- [Use cases](#use-cases)
<!-- /toc -->

## Suspended state

When a resource is suspended, all operations performed by the operator are disabled, including but not limited to:
- Provisioning
- Upgrades
- Volume resize
- Galera cluster recovery

More specifically, the reconciliation loop of the operator is omitted, anything part of it will not happen while the resource is suspended.

## Suspend a resource

Currently, only `MariaDB` and `MaxScale` resources support suspension. You can enable it by setting `suspend=true`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  suspend: true
```

This results in the reconciliation loop being disabled and the status being marked as `Suspended`:

```bash
kubectl get mariadbs
NAME             READY   STATUS      PRIMARY POD        AGE
mariadb-galera   False   Suspended   mariadb-galera-0   25h
```

To re-enable it, simply remove the `suspend` setting or set it to `suspend=false`.

## Use cases

#### Progressive fleet upgrades

If you're managing fleets of thousands of databases, you likely prefer to roll out updates progressively rather than simultaneously across all instances.

#### Operator upgrades

When upgrading `mariadb-operator`, changes to the `StatefulSet` or the `Pod` template may occur from one version to another, which could trigger a rolling update of your `MariaDB` instances.

#### Maintenance

Disabling the reconciliation loop can be useful when manual operations need to be performed on an instance, as it helps prevent conflicts with the operator.