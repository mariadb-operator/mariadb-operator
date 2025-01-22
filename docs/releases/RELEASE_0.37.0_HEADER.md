**`{{ .ProjectName }}` [0.37.0](https://github.com/mariadb-operator/mariadb-operator/releases/tag/0.37.0) is here!** 🦭

We're excited to introduce __[TLS](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/TLS.md)__ 🔐 support in this release, one of the major features of `mariadb-operator` so far! ✨ Check out the __[TLS docs](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/TLS.md)__, our [example catalog](https://github.com/mariadb-operator/mariadb-operator/tree/main/examples/manifests) and the release notes below to start using it.

> [!WARNING]
> Be sure to follow the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_0.37.0.md)__ to ensure a seamless transition from previous versions.

### Issue certificates for `MariaDB` and `MaxScale`

Issuing and configuring TLS certificates for your instances has never been easier, you just need to set `tls.enabled=true`:

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

A self-signed Certificate Authority (CA) will be automatically generated to issue leaf certificates for your instances. The operator will also manage a CA bundle that your applications can use in order to establish trust. 

To ensure security by default, TLS will now be enabled by default. However, you can choose to disable it and use unencrypted connections by setting `tls.enabled=false`.

### Native integration with cert-manager

[cert-manager](https://cert-manager.io/) is the de facto standard for managing certificates in Kubernetes. This certificate controller simplifies the automatic provisioning, management, and renewal of certificates. It supports a variety of [certificate backends](https://cert-manager.io/docs/configuration/issuers/) (e.g. in-cluster, Hashicorp Vault), which are configured using `Issuer` or `ClusterIssuer` resources.

In your `MariaDB` and `MaxScale` resources, you can directly reference `ClusterIssuer` or `Issuer` objects to seamlessly issue certificates:

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

Under the scenes, the operator will create cert-manager's `Certificate` resources with all the required Subject Alternative Names (SANs) required by your instances. These certificates will be automatically managed by cert-manager and the CA bundle will be updated by the operator so you can establish trust with your instances.

The advantage of this approach is that you can use any of the [cert-manager's certificate backends](https://cert-manager.io/docs/configuration/issuers/), such as the in-cluster CA or HashiCorp Vault, and potentially reuse the same `Issuer`/`ClusterIssuer` with multiple instances.

### Certificate rotation

Whether the certificates are managed by the operator or by cert-manager, they will be automatically renewed before expiration. Additionally, the operator will update the CA bundle whenever the CAs are rotated, temporarily retaining the old CA in the bundle to ensure a seamless update process.

In both scenarios, the standard [update strategies](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPDATES.md) apply, allowing you to control how the `Pods` are restarted during certificate rotation.

### TLS requirements for `Users`

We have extended our `User` SQL resource to include TLS-specific requirements for user connections over TLS. For example, if you want to enforce the use of a valid x509 certificate for a user to connect:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: user
spec:
  ...
  require:
    x509: true
```

To restrict the subject of the user's certificate and/or require a specific issuer, you may set:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: user
spec:
  ...
  require:
    issuer: "/CN=mariadb-galera-ca"
    subject: "/CN=mariadb-galera-client"
```

If any of these TLS requirements are not satisfied, the user will be unable to connect to the instance.
 
### Community contributions

- [Support startupProbe in MariaDB and MaxScale](https://github.com/mariadb-operator/mariadb-operator/pull/1053) by @vixns 
- [Operator configuration via helm](https://github.com/mariadb-operator/mariadb-operator/pull/1098) by @sakazuki and @indigo-saito
- [Support EKS Service Accounts in S3](https://github.com/mariadb-operator/mariadb-operator/pull/1115) by @Skaronator
- [Fix examples SqlJob secret reference](https://github.com/mariadb-operator/mariadb-operator/pull/1090) by @driv 

Huge thanks to our awesome contributors! 🙇

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.