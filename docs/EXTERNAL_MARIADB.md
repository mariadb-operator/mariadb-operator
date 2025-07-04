# TLS

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.37

`mariadb-operator` supports external `MariadDB` management (i.e. A MariaDB running outside of the MariaDB Operator). It allows to take backups, manage users, privileges, databases and to run SQLJobs on declarative a way, using same YAML files used to manage internal MariaDB servers.

## Table of contents
<!-- toc -->
- [`ExternalMariaDB` configuration](#externalmariadb-configuration)
- [Supported objects](#supported-objects)
<!-- /toc -->

## `ExternalMariaDB` configuration

> [!IMPORTANT]  
> This section covers ExternalMariaDB configuration.

A `ExternalMariaDB` can be configured similar to `MariaDB` but we need to provide a `host`, `username`
and a secret containing the user password:
```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: external-mariadb
spec:
  host: mariadb-1.example.com
  username: root
  port: 3309
  passwordSecretKeyRef:
    name: mariadb-1-su-secret
    key: password
  connection:
    secretName: external-mariadb-dba-secret
    healthCheck:
      interval: 5s
      retryInterval: 10s
```
As a result, the operator will create an ExtenalMariaDB object and a `connection` object.

If you want/need to use TLS to connect to the external MariaDB, you can provide the server CA certificate and the client certificate sercrets through the `tls` object:
```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: external-mariadb
spec:
  host: mariadb-1.example.com
  username: root
  port: 3306
  tls:
    clientCertSecretRef:
      name: client-cert-secret
    enabled: true
    required: false
    serverCASecretRef:
      name: ca-cert-secret
  passwordSecretKeyRef:
    name: mariadb-1-su-secret
    key: password
  connection:
    secretName: external-mariadb-dba-secret
    healthCheck:
      interval: 5s
      retryInterval: 10s
```
This approach ensures that connections from the operator to the external MariaDB will use TLS.


## Supported objects

Currently the `ExternalMariaDB` is supported by the following objects:
* Connection
* User
* Grant
* Backup
* SQLJob

You can use it as regular `MariaDB` (Internal) definition just seting the `Kind` to `ExternalMariaDB` on the `MariaDBRef` field.

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: user-external
spec:
  mariaDbRef:
    name: external-mariadb
    kind: ExternalMariaDB
  passwordSecretKeyRef:
    name: user-external-secret
    key: password
  # This field defaults to 10
  maxUserConnections: 20
  host: "%"
  cleanupPolicy: Delete
  requeueInterval: 30s
  retryInterval: 5s
```
