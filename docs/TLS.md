# TLS

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.37

`mariadb-operator` supports issuing, configuring and rotating TLS certiticates for both your `MariaDB` and `MaxScale` resource. It aims to be secure by default, for this reason, TLS certificates are issued and configured by the operator as a default behaviour.

Secure by default, TLS enabled by default.

## Table of contents
<!-- toc -->
- [Configuration](#configuration)
- [`MariaDB` CAs and certificates](#mariadb-cas-and-certificates)
- [`MaxScale` CAs and certificates](#maxscale-cas-and-certificates)
- [CA bundle](#ca-bundle)
- [Issuing certificates with mariadb-operator](#issuing-certificates-with-mariadb-operator)
- [Issuing certificates with cert-manager](#issuing-certificates-with-cert-manager)
- [Issuing certificates manually](#issuing-certificates-manually)
- [Bring your own CA](#bring-your-own-ca)
- [Intermediate CAs](#intermediate-cas)
- [Custom trust](#custom-trust)
- [CA renewal](#ca-renewal)
- [Certificate renewal](#certificate-renewal)
- [Certificate status](#certificate-status)
- [TLS requirements for `Users`](#tls-requirements-for-users)
- [Testing TLS with `Connections`](#testing-tls-with-connections)
- [Connecting applications with TLS](#connecting-applications-with-tls)
- [Limitations](#limitations)
<!-- /toc -->

## Configuration

The easieast way to configure TLS in both `MariaDB` and `MaxScale` is by setting `tls.enabled=true`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  ...
  tls:
    enabled: true
```

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale
spec:
  ...
  tls:
    enabled: true
```

By doing so, the operator will issue a CA for each `MariaDB` and `MaxScale` resource, and use it to issue leaf certificates mounted by the workloads. This the default behaviour when no `tls` field is specified. 

You can opt-out from TLS and use unencrypted connections just by setting `tls.enabled=false`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  ...
  tls:
    enabled: false
```

Refer to the following sections for a more advanced TLS configuration.

## `MariaDB` certificate specification

The `MariaDB` TLS setup consists of the following certificates:
- Certificate Authority (CA) keypair to issue the server certificate
- Server leaf certificate: Used to encrypt server connections
- Certificate Authority (CA) keypair to issue the client certificate
- Client leaf certificate: Used to provide as authentication when connecting to the server.

As a default behaviour, the operator issues a single CA to be used for issuing both the server and client certificates, but the user can decide to use dedicated CAs for each case. Root and intermedicate CAs are supported.

The server certificate contains the following Subject Alternative Names (SANs):
- `<mariadb-name>.default.svc.<cluster-name>`
- `<mariadb-name>.default.svc`
- `<mariadb-name>.default`
- `<mariadb-name>`
- `*.<mariadb-name>-internal.default.svc.<cluster-name>`
- `*.<mariadb-name>-internal.default.svc`
- `*.<mariadb-name>-internal.default`
- `*.<mariadb-name>-internal`
- `<mariadb-name>-primary.default.svc.<cluster-name>`
- `<mariadb-name>-primary.default.svc`
- `<mariadb-name>-primary.default`
- `<mariadb-name>-primary`
- `<mariadb-name>-secondary.default.svc.<cluster-name>`
- `<mariadb-name>-secondary.default.svc`
- `<mariadb-name>-secondary.default`
- `<mariadb-name>-secondary`
- `localhost`

Whereas the client certificate is only valid for the `<mariadb-name>-client` SAN.

## `MaxScale` certificate specification

The `MaxScale` TLS setup consists of the following certificates:
- Certificate Authority (CA) keypair to issue the admin certificate.
- Admin certificate: Used to encrypt the administrative REST API and GUI.


By default, single CA issued by the operator.

References `MariaDB` client certificates for simplicity.

## CA bundle

Contains non expired CAs. When renewing a CA, the new CA is appended to the bundle and the old one is kept until expired. This ensures that all certificates issued by the old CA are still valid.

## Issue certificates with mariadb-operator

## Issue certificates with cert-manager

> [!IMPORTANT]
> [cert-manager](https://cert-manager.io/) must be previously installed in the cluster in order to use this feature.

## Provide certificates manually

## Bring your own CA

## Intermediate CAs

## Custom trust

## CA renewal

mariadb-operator and cert-manager

upgrades

## Certificate renewal

mariadb-operator and cert-manager

upgrades

Wait for CA rolling upgrade.

## Certificate status

## TLS requirements for `Users`

## Testing TLS with `Connections`

## Connecting applications with TLS

## Limitations