# TLS

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.37

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

## `MariaDB` CAs and certificates

By default, single CA issued by the operator.

## `MaxScale` CAs and certificates

By default, single CA issued by the operator.

References `MariaDB` client certificates for simplicity.

## CA bundle

Contains non expired CAs. When renewing a CA, the new CA is appended to the bundle and the old one is kept until expired. This ensures that all certificates issued by the old CA are still valid.

## Issuing certificates with mariadb-operator

## Issuing certificates with cert-manager

> [!IMPORTANT]
> [cert-manager](https://cert-manager.io/) must be previously installed in the cluster in order to use this feature.

## Issuing certificates manually

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