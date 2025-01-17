# TLS

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.37

Secure by default, TLS enabled by default.

## Table of contents
<!-- toc -->
<!-- /toc -->

## `MariaDB` CAs and certificates

By default, single CA issued by the operator.

## `MaxScale` CAs and certificates

By default, single CA issued by the operator.

References `MariaDB` client certificates for simplicity.

## CA bundle

Contains non expired CAs. When renewing a CA, the new CA is appended to the bundle and the old one is kept until expired. This ensures that all certificates issued by the old CA are still valid.

## Issuing certificates with mariadb-operator

## Issuing certificates with cert-manager

## TLS requirements for `Users`

## Connecting applications with TLS

## Certificate status

## CA renewal

## Certificate renewal

Wait for CA rolling upgrade.

## Galera limitations