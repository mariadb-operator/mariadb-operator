# External MariaDB

`mariadb-operator` supports managing resources in external MariaDB instances i.e running outside of the Kubernetes cluster where the operator runs. It allows to take backups, manage users, privileges, databases and to run SQL jobs declaratively, using the same CRs that you use to manage internal MariaDB instances

## Table of contents
<!-- toc -->
- [`ExternalMariaDB` configuration](#externalmariadb-configuration)
- [Supported objects](#supported-objects)
<!-- /toc -->

## `ExternalMariaDB` configuration

The `ExternalMariaDB` resource is similar to the internal `MariaDB` resource, but we need to provide a `host`, `username` and a reference to a `Secret` containing the user password. These will be the connection details that the operator will use to connect to the external MariaDB in order to manage resources, make sure that the specified user has enough privileges:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: external-mariadb
spec:
  host: mariadb.default.svc.cluster.local
  port: 3306
  username: root
  passwordSecretKeyRef:
    name: mariadb
    key: password
  connection:
    secretName: external-mariadb
    healthCheck:
      interval: 5s
```
If you need to use TLS to connect to the external MariaDB, you can provide the server CA certificate and the client certificate sercrets via the `tls` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: external-mariadb
spec:
  host: mariadb.default.svc.cluster.local
  port: 3306
  username: root
  passwordSecretKeyRef:
    name: mariadb
    key: password
  tls:
    enabled: true
    clientCertSecretRef:
      name: client-cert-secret
    serverCASecretRef:
      name: ca-cert-secret
  connection:
    secretName: external-mariadb-dba-secret
    healthCheck:
      interval: 5s
      retryInterval: 10s
```
As a result, you will be able to specify the `ExternalMariaDB` as a reference in [multiple objects](#supported-objects), the same way you would do for a internal `MariaDB` resource.

As part of the `ExternalMariaDB` reconciliation, a `Connection` will be created whenever the `connection` template is specified. This could be handy to track the external connection status and declaratively create a connection string in a `Secret` to be consumed by applications to connect to the external `MariaDB`.

## Supported objects

Currently, the `ExternalMariaDB` resource is supported by the following objects:
- `Connection`
- `User`
- `Grant`
- `Database`
- `Backup`
- `SqlJob`

You can use it as an internal `MariaDB` resource, just by setting `kind` to `ExternalMariaDB` in the `mariaDBRef` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: user-external
spec:
  name: user
  mariaDbRef:
    name: external-mariadb
    kind: ExternalMariaDB
  passwordSecretKeyRef:
    name: mariadb
    key: password
  maxUserConnections: 20
  host: "%"
  cleanupPolicy: Delete
  requeueInterval: 10h
  retryInterval: 30s
```

When the previous example gets reconciled, an user will be created in the referred external MariaDB instance.