# TLS

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.37

`mariadb-operator` supports issuing, configuring and rotating TLS certiticates for both your `MariaDB` and `MaxScale` resources. It aims to be secure by default, for this reason, TLS certificates are issued and configured by the operator as a default behaviour.

## Table of contents
<!-- toc -->
- [Configuration](#configuration)
- [`MariaDB` certificate specification](#mariadb-certificate-specification)
- [`MaxScale` certificate specification](#maxscale-certificate-specification)
- [CA bundle](#ca-bundle)
- [Issue certificates with mariadb-operator](#issue-certificates-with-mariadb-operator)
- [Issue certificates with cert-manager](#issue-certificates-with-cert-manager)
- [Provide certificates manually](#provide-certificates-manually)
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

By doing so, the operator will issue a CA for each `MariaDB` and `MaxScale` resource, and use it to issue leaf certificates mounted by the instances. This also the default behaviour when no `tls` field is specified. 

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
- Client leaf certificate: Used to encrypt and authenticate client connections.

As a default behaviour, the operator issues a single CA to be used for issuing both the server and client certificates, but the user can decide to use dedicated CAs for each case. Root CAs, and [intermedicate CAs](#intermediate-cas) in some cases,  are supported, see [limitations](#intermediate-cas) for further detail. 

The server certificate contains the following Subject Alternative Names (SANs):
- `<mariadb-name>.<namespace>.svc.<cluster-name>`  
- `<mariadb-name>.<namespace>.svc`  
- `<mariadb-name>.<namespace>`  
- `<mariadb-name>`  
- `*.<mariadb-name>-internal.<namespace>.svc.<cluster-name>`  
- `*.<mariadb-name>-internal.<namespace>.svc`  
- `*.<mariadb-name>-internal.<namespace>`  
- `*.<mariadb-name>-internal`  
- `<mariadb-name>-primary.<namespace>.svc.<cluster-name>`  
- `<mariadb-name>-primary.<namespace>.svc`  
- `<mariadb-name>-primary.<namespace>`  
- `<mariadb-name>-primary`  
- `<mariadb-name>-secondary.<namespace>.svc.<cluster-name>`  
- `<mariadb-name>-secondary.<namespace>.svc`  
- `<mariadb-name>-secondary.<namespace>`  
- `<mariadb-name>-secondary`
- `localhost`

Whereas the client certificate is only valid for the `<mariadb-name>-client` SAN.

## `MaxScale` certificate specification

The `MaxScale` TLS setup consists of the following certificates:
- Certificate Authority (CA) keypair to issue the admin certificate.
- Admin leaf certificate: Used to encrypt the administrative REST API and GUI.
- Certificate Authority (CA) keypair to issue the listener certificate.
- Listener leaf certificate: Used to encrypt database connections to the listener.
- Server CA bundle: Used to establish trust with the MariaDB server.
- Server leaf certificate: Used to connect to the MariaDB server.

As a default behaviour, the operator issues a CA to be used for issuing both the admin and the listener certificates, but the user can decide use dedicated CAs for each case. Client certificate and CA bundle configured in the referred MariaDB are used as Server certificates by default, but the user is able to provide its own certificates. Root CAs, and [intermedicate CAs](#intermediate-cas) in some cases,  are supported, see [limitations](#intermediate-cas) for further detail.

Both the admin and listener certificates contain the following Subject Alternative Names (SANs):
- `<maxscale-name>.<namespace>.svc.<clusername>`  
- `<maxscale-name>.<namespace>.svc`  
- `<maxscale-name>.<namespace>`  
- `<maxscale-name>`  
- `<maxscale-name>-gui.<namespace>.svc.<clusername>`  
- `<maxscale-name>-gui.<namespace>.svc`  
- `<maxscale-name>-gui.<namespace>`  
- `<maxscale-name>-gui`  
- `*.<maxscale-name>-internal.<namespace>.svc.<clusername>`  
- `*.<maxscale-name>-internal.<namespace>.svc`  
- `*.<maxscale-name>-internal.<namespace>`  
- `*.<maxscale-name>-internal`

For details about the server certificate, see [`MariaDB` certificate specification](#mariadb-certificate-specification).


## CA bundle

As you could appreciate in [`MariaDB` certificate specification](#mariadb-certificate-specification) and [`MaxScale` certificate specification](#maxscale-certificate-specification), the TLS setup involves multiple CAs. In order to establish trust in a more convenient way, the operator groups the CAs together in a CA bundle that will need to be specified when [securely connecting from your applications](#connecting-applications-with-tls). Every `MariaDB` and `MaxScale` resources have a dedicated bundle of its own available in a `Secret` named `<instance-name>-ca-bundle`. 

These trust bundles contain the non expired CAs needed to connect to the instances. New CAs are automatically added to the bundle after [renewal](#ca-renewal), whilst old CAs will be removed after they expire. It is important to note that both the new and old CA will remain in the bundle for a while to ensure a smooth rolling upgrade when the new certificates are issued by the new CA.

## Issue certificates with mariadb-operator

By setting `tls.enabled=true`, mariadb-operator will generate a root CA for each instance, which will be used to issue the certificates described in the [`MariaDB` cert spec](#mariadb-certificate-specification) and [`MaxScale` cert spec](#maxscale-certificate-specification) sections:

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

To establish trust with the instances, the public key of the CA will be added to the [CA bundle](#ca-bundle). If you need a different trust chain, please refer to the [custom trust](#custom-trust) section.

The advantage of this approach is the operator fully manages the `Secrets` that contain the certificates without depending on any third party dependency.

## Issue certificates with cert-manager

> [!IMPORTANT]
> [cert-manager](https://cert-manager.io/) must be previously installed in the cluster in order to use this feature.

cert-manager is the de-facto standard for managing certificates in Kubernetes. It is a Kubernetes native certificate management controller that allows you to automatically provision, manage, and renew certificates. It supports multiple certificate backends, which are configured as `Issuers` or `ClusterIssuers`.

As an example, we are going to setup an in-cluster root CA `ClusterIssuer`:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: root-ca
  namespace: default
spec:
  duration: 52596h # 6 years
  commonName: root-ca
  usages:
  - digital signature
  - key encipherment
  - cert sign
  issuerRef:
    name: selfsigned
    kind: ClusterIssuer
  isCA: true
  privateKey:
    encoding: PKCS1
    algorithm: ECDSA
    size: 256
  secretTemplate:
    labels:
      k8s.mariadb.com/watch: ""
  secretName: root-ca
  revisionHistoryLimit: 10
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: root-ca
spec:
  ca:
    secretName: root-ca
```

Then, you can reference the `ClusterIssuer` in the `MariaDB` and `MaxScale` resources:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  tls:
    enabled: true
    serverCertIssuerRef:
      name: root-ca
      kind: ClusterIssuer
    clientCertIssuerRef:
      name: root-ca
      kind: ClusterIssuer
```
```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  ...
  tls:
    enabled: true
    adminCertIssuerRef:
      name: root-ca
      kind: ClusterIssuer
    listenerCertIssuerRef:
      name: root-ca
      kind: ClusterIssuer
``` 

The operator will create cert-manager's [`Certificate` resources](https://cert-manager.io/docs/reference/api-docs/#cert-manager.io/v1.Certificate) for each certificate, and will mount the resulting certificates in the instances. The TLS `Secrets` containing the certificates will be managed by cert-manager as well as its renewal process.

To establish trust with the instances, the `ca.crt` field provided by cert-managed in the certificate `Secret` will be added to the [CA bundle](#ca-bundle). If you need a different trust chain, please refer to the [custom trust](#custom-trust) section.

The advantage of this approach is that you can easily reuse the same CA for multiple resources, and make use any of the supported certificate backends, such as HashiCorp Vault or Let's Encrypt.

## Provide certificates manually

CA Secret structure
TLS secret structure

## Bring your own CA

CA Secret structure

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