# Connections

`mariadb-operator` provides the `Connection` resource to configure connection strings for applications connecting to MariaDB. This resource creates and maintains a Kubernetes `Secret` containing the credentials and connection details needed by your applications.

## Table of contents
<!-- toc -->
- [`Connection` CR](#connection-cr)
- [Credential generation](#credential-generation)
- [Secret template](#secret-template)
- [Custom DSN format](#custom-dsn-format)
- [TLS authentication](#tls-authentication)
- [Cross-namespace connections](#cross-namespace-connections)
- [MaxScale connections](#maxscale-connections)
- [External MariaDB connections](#external-mariadb-connections)
- [Embedded Connection template](#embedded-connection-template)
- [Health checking](#health-checking)
- [Reference](#reference)
<!-- /toc -->

## `Connection` CR

A `Connection` resource declares an intent to create a connection string for applications to connect to a MariaDB instance. When reconciled, it creates a `Secret` containing the DSN and individual connection parameters:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection
spec:
  mariaDbRef:
    name: mariadb
  username: mariadb
  passwordSecretKeyRef:
    name: mariadb
    key: password
  database: mariadb
  secretName: connection
  healthCheck:
    interval: 30s
    retryInterval: 3s
```

The operator creates a `Secret` named `connection` containing a DSN and individual fields like `username`, `password`, `host`, `port`, and `database`. Applications can mount this `Secret` to obtain the connection details.

## Credential generation

`mariadb-operator` can automatically generate credentials for users via the `GeneratedSecretKeyRef` type with the `generate: true` field. This feature is available in the `MariaDB`, `MaxScale`, and `User` resources.

For example, when creating a `MariaDB` resource with an initial user:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  username: app
  passwordSecretKeyRef:
    name: app-password
    key: password
    generate: true
  database: app
```

The operator will automatically generate a random password and store it in a `Secret` named `app-password`. You can then reference this `Secret` in your `Connection` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: app-connection
spec:
  mariaDbRef:
    name: mariadb
  username: app
  passwordSecretKeyRef:
    name: app-password
    key: password
  database: app
  secretName: app-connection
```

If you prefer to provide your own password, you can opt-out from random password generation by either not providing the `generate` field or setting it to `false`. This enables the use of GitOps tools like [sealed-secrets](https://github.com/bitnami-labs/sealed-secrets) or [external-secrets](https://github.com/external-secrets/external-secrets) to seed the password.

## Secret template

The `secretTemplate` field allows you to customize the output `Secret`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection
spec:
  mariaDbRef:
    name: mariadb
  username: mariadb
  passwordSecretKeyRef:
    name: mariadb
    key: password
  database: mariadb
  secretName: connection
  secretTemplate:
    metadata:
      labels:
        app.kubernetes.io/name: myapp
      annotations:
        app.kubernetes.io/managed-by: mariadb-operator
    key: dsn
    usernameKey: username
    passwordKey: password
    hostKey: host
    portKey: port
    databaseKey: database
```

The resulting `Secret` will contain:
- `dsn`: The full connection string
- `username`: The database username
- `password`: The database password
- `host`: The database host
- `port`: The database port
- `database`: The database name

## Custom DSN format

You can customize the DSN format using Go templates via the `format` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection
spec:
  mariaDbRef:
    name: mariadb
  username: mariadb
  passwordSecretKeyRef:
    name: mariadb
    key: password
  database: mariadb
  params:
    parseTime: "true"
    timeout: "5s"
  secretName: connection
  secretTemplate:
    key: dsn
    format: mysql://{{ .Username }}:{{ .Password }}@{{ .Host }}:{{ .Port }}/{{ .Database }}{{ .Params }}
```

Available template variables:
- `{{ .Username }}`: The database username
- `{{ .Password }}`: The database password
- `{{ .Host }}`: The database host
- `{{ .Port }}`: The database port
- `{{ .Database }}`: The database name
- `{{ .Params }}`: Query parameters (e.g., `?parseTime=true&timeout=5s`)

## TLS authentication

`Connection` supports TLS client certificate authentication as an alternative to password authentication:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: app
spec:
  mariaDbRef:
    name: mariadb-galera
  require:
    issuer: "/CN=mariadb-galera-ca"
    subject: "/CN=mariadb-galera-client"
  host: "%"
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: Grant
metadata:
  name: grant-app
spec:
  mariaDbRef:
    name: mariadb-galera
  privileges:
    - "ALL PRIVILEGES"
  database: "*"
  table: "*"
  username: app
  host: "%"
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection
spec:
  mariaDbRef:
    name: mariadb-galera
  username: app
  tlsClientCertSecretRef:
    name: mariadb-galera-client-cert
  healthCheck:
    interval: 30s
```

When using TLS authentication, provide `tlsClientCertSecretRef` instead of `passwordSecretKeyRef`. The referenced `Secret` must be a Kubernetes TLS `Secret` containing the client certificate and key.

## Cross-namespace connections

`Connection` resources can reference `MariaDB` instances in different namespaces:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection
  namespace: app
spec:
  mariaDbRef:
    name: mariadb
    namespace: mariadb
  username: app
  passwordSecretKeyRef:
    name: app
    key: password
  database: app
  secretName: connection
```

This creates a `Connection` in the `app` namespace that references a `MariaDB` in the `mariadb` namespace.

## MaxScale connections

`Connection` resources can reference `MaxScale` instances using `maxScaleRef`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection-maxscale
spec:
  maxScaleRef:
    name: maxscale-galera
  username: maxscale-galera-client
  passwordSecretKeyRef:
    name: maxscale-galera-client
    key: password
  secretName: conn-mxs
  port: 3306
  healthCheck:
    interval: 30s
```

When referencing a `MaxScale`, the operator uses the MaxScale listener port. The health check will consume connections from the MaxScale connection pool.

## External MariaDB connections

`Connection` resources can reference `ExternalMariaDB` instances by specifying `kind: ExternalMariaDB` in the `mariaDbRef`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection-external
spec:
  mariaDbRef:
    name: external-mariadb
    kind: ExternalMariaDB
  username: user
  passwordSecretKeyRef:
    name: mariadb
    key: password
  database: mariadb
  secretName: connection-external
  healthCheck:
    interval: 5s
```

This is useful for generating connection strings to external MariaDB instances running outside of Kubernetes.

## Embedded Connection template

Instead of creating a separate `Connection` resource, you can embed a connection template directly in the `MariaDB`, `MaxScale`, or `ExternalMariaDB` resources:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  username: mariadb
  database: mariadb

  connection:
    secretName: connection-mariadb
    secretTemplate:
      key: dsn
    healthCheck:
      interval: 10s
      retryInterval: 3s
    params:
      parseTime: "true"

  storage:
    size: 1Gi
```

When the `connection` template is specified, the operator automatically creates a `Connection` resource as part of the parent resource reconciliation. This is convenient when you need a single connection string for the initial user.

## Health checking

The `healthCheck` field configures periodic health checks to verify database connectivity:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection
spec:
  mariaDbRef:
    name: mariadb
  username: mariadb
  passwordSecretKeyRef:
    name: mariadb
    key: password
  database: mariadb
  secretName: connection
  healthCheck:
    interval: 30s
    retryInterval: 3s
```

- `interval`: How often to perform health checks (default: 30s)
- `retryInterval`: How often to retry after a failed health check (default: 3s)

The `Connection` status reflects the health check results, allowing you to monitor connectivity issues through Kubernetes.

## Reference
- [API reference](./api_reference.md)
- [Example suite](../examples/)
