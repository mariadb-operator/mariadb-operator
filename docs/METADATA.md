# Metadata

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.28

This documentation shows how to configure metadata in the `mariadb-operator` CRs.

## Children object metadata

`MariaDB` and `MaxScale` resources allow you to propagate metadata to all the children objects by specifying the `inheritMetadata` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  inheritMetadata:
    labels:
      database.myorg.io: mariadb
    annotations:
      database.myorg.io: mariadb
...
```

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  inheritMetadata:
    labels:
      database.myorg.io: maxscale
    annotations:
      database.myorg.io: maxscale
...
```

This means that all the reconciled object will inherit these labels and annotations. For instance, see the `Services` and `Pods`:

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    database.myorg.io: mariadb
  labels:
    app.kubernetes.io/instance: mariadb
    app.kubernetes.io/name: mariadb
    database.myorg.io: mariadb
  name: mariadb-galera-primary
  namespace: default
``` 

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    database.myorg.io: maxscale
  labels:
    app.kubernetes.io/instance: maxscale-galera
    app.kubernetes.io/name: maxscale
    database.myorg.io: maxscale
  name: maxscale-galera-0
  namespace: default
``` 

## `Pod` metadata

Override

## `Service` metadata

Override

## `PVC` metadata

## Use cases

Istio -> Pod
Metallb -> Services