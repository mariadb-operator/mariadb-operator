# SQL resources

`mariadb-operator` enables you to manage SQL resources declaratively through CRs. By SQL resources, we refer to users, grants, and databases that are typically created using SQL statements.

The key advantage of this approach is that, unlike executing SQL statements, which is a one-time action, declaring a SQL resource via a CR ensures that the resource is periodically reconciled by the operator. This provides a guarantee that the resource will be recreated if it gets manually deleted. Additionally, it helps prevent state drifts, as the operator will regularly update the resource according to the CR specification.

## Table of contents
<!-- toc -->
- [`User` CR](#user-cr)
- [`Grant` CR](#grant-cr)
- [`Database` CR](#database-cr)
- [Initial `User`, `Grant` and `Database`](#initial-user-grant-and-database)
- [Authentication plugins](#authentication-plugins)
- [Configure reconciliation](#configure-reconciliation)
- [Cleanup policy](#cleanup-policy)
- [Reference](#reference)
<!-- /toc -->

## `User` CR

By creating this resource, you are declaring an intent to create an user in the referred `MariaDB` instance, just like a [`CREATE USER`](https://mariadb.com/kb/en/create-user/) statement would do:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: bob
spec:
  mariaDbRef:
    name: mariadb
  passwordSecretKeyRef:
    name: bob-password
    key: password
  maxUserConnections: 20
  host: "%"
  cleanupPolicy: Delete  
```

In the example above, a user named `bob` identified by the password available in the `bob-password` `Secret` will be created in the `mariadb` instance.

Refer to the [reference section](#reference) for more detailed information about every field.

#### Custom name

By default, the CR name is used to create the user in the database, but you can specify a different one providing the `name` field under spec:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: user
spec:
  name: user-custom
```

## `Grant` CR

By creating this resource, you are declaring an intent to grant permissions to a given user in the referred `MariaDB` instance, just like a [`GRANT`](https://mariadb.com/kb/en/grant/) statement would do.

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Grant
metadata:
  name: grant-bob
spec:
  mariaDbRef:
    name: mariadb
  privileges:
    - "SELECT"
    - "INSERT"
    - "UPDATE"
  database: "*"
  table: "*"
  username: bob
  grantOption: true
  host: "%"
```
You may provide any set of [privileges supported by MariaDB](https://mariadb.com/kb/en/grant/#privilege-levels).

Refer to the [reference section](#reference) for more detailed information about every field.

## `Database` CR

By creating this resource, you are declaring an intent to create a logical database in the referred `MariaDB` instance, just like a [`CREATE DATABASE`](https://mariadb.com/kb/en/create-database/) statement would do:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Database
metadata:
  name: wordpress
spec:
  mariaDbRef:
    name: mariadb
  characterSet: utf8
  collate: utf8_general_ci
```
Refer to the [reference section](#reference) for more detailed information about every field.

#### Custom name

By default, the CR name is used to create the user in the database, but you can specify a different one providing the `name` field under spec:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Database
metadata:
  name: database
spec:
  name: database-custom
```

## Initial `User`, `Grant` and `Database`

If you only need one user to interact with a single logical database, you can make use of the `MariaDB` resource to configure it, instead of creating the `User`, `Grant` and `Database` resources separately:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  username: bob
  passwordSecretKeyRef:
    name: bob-password
    key: password
  database: wordpress
``` 

Behind the scenes, the operator will be creating an `User` resource with `ALL PRIVILEGES` in the initial `Database`. 

## Authentication plugins

Passwords can be supplied using the `passwordSecretKeyRef` field in the `User` CR. This is a reference to a `Secret` that containers password in plain text. 

Alternatively, you can use [MariaDB authentication plugins](https://mariadb.com/kb/en/authentication-plugins/) to avoid storing passwords in plain text and provide the password in a hashed format instead. This doesn't affect the end user experience, they will still need to provide the password in plain text to authenticate.

#### Password hash

Provide the password hashed using the [MariaDB `PASSWORD` ](https://mariadb.com/kb/en/password/)function:

```yaml
kind: Secret
metadata:
  name: mariadb-auth
stringData:
  passwordHash: "*57685B4F0FF9D049082E296E2C39354B7A98774E"
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: user-password-hash
spec:
  mariaDbRef:
    name: mariadb
  passwordHashSecretKeyRef:
    name: mariadb-auth
    key: passwordHash
  host: "%"
```

The password hash can be obtaned by executing `SELECT PASSWORD('<password>');` in an existing MariaDB installation.

#### Password plugin

Provide the password hashed using any of the available [MariaDB authentication plugins](https://mariadb.com/kb/en/authentication-plugins/), for example `mysql_native_password`:

```yaml
kind: Secret
metadata:
  name: mariadb-auth
stringData:
  passwordHash: "*57685B4F0FF9D049082E296E2C39354B7A98774E"
  nativePasswordPlugin: mysql_native_password
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: user-password-plugin
spec:
  mariaDbRef:
    name: mariadb
  passwordPlugin:
    pluginNameSecretKeyRef:
        name: mariadb-auth
        key: nativePasswordPlugin
    pluginArgSecretKeyRef:
        name: mariadb-auth
        key: passwordHash
  host: "%"
```

The plugin name should be available in a `Secret` referenced by `pluginNameSecretKeyRef` and the argument passed to it in `pluginArgSecretKeyRef`. The argument is the hashed password in most cases, refer to the [MariaDB docs](https://mariadb.com/kb/en/authentication-plugins/) for further detail.

## Configure reconciliation

As we previously mentioned, SQL resources are periodically reconciled by the operator into SQL statements. You are able to tweak the reconciliation interval using the following fields:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: user
spec:
  requeueInterval: 30s
  retryInterval: 5s
```

If the SQL statement executed by the operator is successful, it will schedule the next reconciliation cycle using the `requeueInterval`. If the statement encounters an error, the operator will use the `retryInterval` instead.

## Cleanup policy

Whenever you delete a SQL resource, the operator will also delete the associated resource in the database. This is the default behaviour, that can also be achieved by setting `cleanupPolicy=Delete`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: user
spec:
  cleanupPolicy: Delete
```

You can opt-out from this cleanup process using `cleanupPolicy=Skip`. Note that this resources will remain in the database.

## Reference
- [API reference](./API_REFERENCE.md)
- [Example suite](../examples/)
