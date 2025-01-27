# TLS

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.37

`mariadb-operator` supports issuing, configuring and rotating TLS certiticates for both your `MariaDB` and `MaxScale` resources. It aims to be secure by default, for this reason, TLS certificates are issued and configured by the operator as a default behaviour.

## Table of contents
<!-- toc -->
- [`MariaDB` configuration](#mariadb-configuration)
- [`MaxScale` configuration](#maxscale-configuration)
- [`MariaDB` certificate specification](#mariadb-certificate-specification)
- [`MaxScale` certificate specification](#maxscale-certificate-specification)
- [CA bundle](#ca-bundle)
- [Issue certificates with mariadb-operator](#issue-certificates-with-mariadb-operator)
- [Issue certificates with cert-manager](#issue-certificates-with-cert-manager)
- [Provide your own certificates](#provide-your-own-certificates)
- [Bring your own CA](#bring-your-own-ca)
- [Intermediate CAs](#intermediate-cas)
- [Custom trust](#custom-trust)
- [Distributing trust](#distributing-trust)
- [CA renewal](#ca-renewal)
- [Certificate renewal](#certificate-renewal)
- [Certificate status](#certificate-status)
- [TLS requirements for `Users`](#tls-requirements-for-users)
- [Secure application connections with TLS](#secure-application-connections-with-tls)
- [Test TLS certificates with `Connections`](#test-tls-certificates-with-connections)
- [Enabling TLS in existing instances](#enabling-tls-in-existing-instances)
- [Limitations](#limitations)
<!-- /toc -->

## `MariaDB` configuration

> [!IMPORTANT]  
> This section covers TLS configuration in new instances. If you are looking to migrate an existing instance to use TLS, please refer to [Enabling TLS in existing instances](#enabling-tls-in-existing-instances) instead.

TLS can be configured in `MariaDB` resources by setting `tls.enabled=true`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  tls:
    enabled: true
```
As a result, the operator will generate a Certificate Authority (CA) and use it to issue the leaf certificates mounted by the instance. It is important to note that the TLS connections are not enforced in this case i.e. both TLS and non-TLS connections will be accepted. This is the default behaviour when no `tls` field is specified.

If you want to enforce TLS connections, you can set `tls.required=true`:
```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  tls:
    enabled: true
    required: true
```
This approach ensures that any unencrypted connection will fail, effectively enforcing security best practices.

If you want to fully opt-out from TLS, you can set `tls.enabled=false`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  tls:
    enabled: false
```

This will disable certificate issuance, resulting in all connections being unencrypted.

Refer to further sections for a more advanced TLS configuration.

## `MaxScale` configuration

> [!IMPORTANT]  
> This section covers TLS configuration in new instances. If you are looking to migrate an existing instance to use TLS, please refer to [Enabling TLS in existing instances](#enabling-tls-in-existing-instances) instead.

TLS will be automatically enabled in `MaxScale` when the referred `MariaDB` (via `mariaDbRef`) has TLS enabled and enforced. Alternatively, you can explicitly enable TLS by setting `tls.enabled=true`:
```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  ...
  mariaDbRef:
    name: mariadb-galera
  tls:
    enabled: true
```

As a result, the operator will generate a Certificate Authority (CA) and use it to issue the leaf certificates mounted by the instance. It is important to note that, unlike `MariaDB`, `MaxScale` does not support TLS and non-TLS connections simultaneously (see [limitations](#limitations)). Therefore, TLS connections will be enforced in this case i.e. unencrypted connections will fail, ensuring security best practises.

If you want to fully opt-out from TLS, you can set `tls.enabled=false`. This should only be done when `MariaDB` TLS is not enforced or disabled:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  ...
  mariaDbRef:
    name: mariadb-galera
  tls:
    enabled: false
```

This will disable certificate issuance, resulting in all connections being unencrypted.

Refer to further sections for a more advanced TLS configuration.

## `MariaDB` certificate specification

The `MariaDB` TLS setup consists of the following certificates:
- Certificate Authority (CA) keypair to issue the server certificate.
- Server leaf certificate used to encrypt server connections.
- Certificate Authority (CA) keypair to issue the client certificate.
- Client leaf certificate used to encrypt and authenticate client connections.

As a default behaviour, the operator generates a single CA to be used for issuing both the server and client certificates, but the user can decide to use dedicated CAs for each case. Root CAs, and [intermedicate CAs](#intermediate-cas) in some cases, are supported, see [limitations](#limitations) for further detail. 

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
- Admin leaf certificate used to encrypt the administrative REST API and GUI.
- Certificate Authority (CA) keypair to issue the listener certificate.
- Listener leaf certificate used to encrypt database connections to the listener.
- Server CA bundle used to establish trust with the MariaDB server.
- Server leaf certificate used to connect to the MariaDB server.

As a default behaviour, the operator generates a single CA to be used for issuing both the admin and the listener certificates, but the user can decide to use dedicated CAs for each case. Client certificate and CA bundle configured in the referred MariaDB are used as server certificates by default, but the user is able to provide its own certificates. Root CAs, and [intermedicate CAs](#intermediate-cas) in some cases, are supported, see [limitations](#limitations) for further detail.

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

As you could appreciate in [`MariaDB` certificate specification](#mariadb-certificate-specification) and [`MaxScale` certificate specification](#maxscale-certificate-specification), the TLS setup involves multiple CAs. In order to establish trust in a more convenient way, the operator groups the CAs together in a CA bundle that will need to be specified when [securely connecting from your applications](#connect-applications-with-tls). Every `MariaDB` and `MaxScale` resources have a dedicated bundle of its own available in a `Secret` named `<instance-name>-ca-bundle`. 

These trust bundles contain non expired CAs needed to connect to the instances. New CAs are automatically added to the bundle after [renewal](#ca-renewal), whilst old CAs are removed after they expire. It is important to note that both the new and old CAs remain in the bundle for a while to ensure a smooth update when the new certificates are issued by the new CA.

## Issue certificates with mariadb-operator

By setting `tls.enabled=true`, the operator will generate a root CA for each instance, which will be used to issue the certificates described in the [`MariaDB` cert spec](#mariadb-certificate-specification) and [`MaxScale` cert spec](#maxscale-certificate-specification) sections:

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

To establish trust with the instances, the CA's public key will be added to the [CA bundle](#ca-bundle). If you need a different trust chain, please refer to the [custom trust](#custom-trust) section.

The advantage of this approach is that the operator fully manages the `Secrets` that contain the certificates without depending on any third party dependency. Also, since the operator fully controls the renewal process, it is able to pause a leaf certificate renewal if the CA is being updated at that moment, as described in the [cert renewal](#certificate-renewal) section. 

## Issue certificates with cert-manager

> [!IMPORTANT]
> [cert-manager](https://cert-manager.io/) must be previously installed in the cluster in order to use this feature.

cert-manager is the de-facto standard for managing certificates in Kubernetes. It is a Kubernetes native certificate management controller that allows you to automatically provision, manage and renew certificates. It supports multiple [certificate backends](https://cert-manager.io/docs/configuration/issuers/) (in-cluster, Hashicorp Vault...) which are configured as `Issuer` or `ClusterIssuer` resources.

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

The operator will create cert-manager's [`Certificate` resources](https://cert-manager.io/docs/reference/api-docs/#cert-manager.io/v1.Certificate) for each certificate, and will mount the resulting [TLS `Secrets`](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets) in the instances. These `Secrets` containing the certificates will be managed by cert-manager as well as its renewal process.

To establish trust with the instances, the [`ca.crt` field provided by cert-managed](https://cert-manager.io/docs/faq/#why-isnt-my-certificates-chain-in-my-issued-secrets-cacrt) in the `Secret` will be added to the [CA bundle](#ca-bundle). If you need a different trust chain, please refer to the [custom trust](#custom-trust) section.

The advantage of this approach is that you can use any of the [cert-manager's certificate backends](https://cert-manager.io/docs/configuration/issuers/), such as the in-cluster CA or HashiCorp Vault, and potentially reuse the same `Issuer`/`ClusterIssuer` with multiple instances.

## Provide your own certificates

Providing your own certificates is as simple as creating the `Secrets` with the appropriate structure and referencing them in the `MariaDB` and `MaxScale` resources. The certificates must be compliant with the [`MariaDB` cert spec](#mariadb-certificate-specification) and [`MaxScale` cert spec](#maxscale-certificate-specification).

The CA certificate must be provided as a `Secret` with the following structure:
```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: mariadb-galera-server-ca
  labels:
    k8s.mariadb.com/watch: ""
data:
  ca.crt:
  -----BEGIN CERTIFICATE-----
  <public-key>
  -----END CERTIFICATE-----
  ca.key:
  -----BEGIN EC PRIVATE KEY-----
  <private-key>
  -----END EC PRIVATE KEY-----
```

The `ca.key` field is only required if you want to the operator to automatically re-issue certificates with this CA, see [bring your own CA](#bring-your-own-ca) for further detail. In other words, if only `ca.crt` is provided, the operator will trust this CA by adding it to the [CA bundle](#ca-bundle), but no certificates will be issued with it, the user will responsible for upating the certificate `Secret` manually with renewed certificates.

The `k8s.mariadb.com/watch` label is required only if you want the operator to automatically trigger an update when the CA is renewed, see [CA renewal](#ca-renewal) for more detail.

The leaf certificate must match the previous CA's public key, and it should provided as a [TLS `Secret`](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets) with the following structure:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls  
metadata:
  name: mariadb-galera-server-tls 
  labels:
    k8s.mariadb.com/watch: ""
data:
  tls.crt:
  -----BEGIN CERTIFICATE-----
  <public-key>
  -----END CERTIFICATE-----
  tls.key:
  -----BEGIN EC PRIVATE KEY-----
  <private-key>
  -----END EC PRIVATE KEY-----
```

The `k8s.mariadb.com/watch` label is required only if you want the operator to automatically trigger an update when the certificate is renewed, see [cert renewal](#certificate-renewal) for more detail.

Once the certificate `Secrets` are available in the cluster, you can create the `MariaDB` and `MaxScale` resources referencing them:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  tls:
    enabled: true
    serverCASecretRef:
      name: mariadb-server-ca
    serverCertSecretRef:
      name: mariadb-galera-server-tls
    clientCASecretRef:
      name: mariadb-client-ca
    clientCertSecretRef:
      name: mariadb-galera-client-tls
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
    adminCASecretRef:
      name: maxscale-admin-ca
    adminCertSecretRef:
      name: maxscale-galera-admin-tls
    listenerCASecretRef:
      name: maxscale-listener-ca
    listenerCertSecretRef:
      name: maxscale-galera-listener-tls
    serverCASecretRef:
      name: mariadb-galera-ca-bundle
    serverCertSecretRef:
      name: mariadb-galera-client-tls
``` 

## Bring your own CA

If you already have a CA setup outside of Kubernetes, you can use it with the operator by providing the CA certificate as a `Secret` with the following structure:

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: mariadb-ca
  labels:
    k8s.mariadb.com/watch: ""
data:
  ca.crt:
  -----BEGIN CERTIFICATE-----
  <public-key>
  -----END CERTIFICATE-----
  ca.key:
  -----BEGIN EC PRIVATE KEY-----
  <private-key>
  -----END EC PRIVATE KEY-----
```

Just by providing a reference to this `Secret`, the operator will use it to issue leaf certificates instead of generating a new CA:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  tls:
    enabled: true
    serverCASecretRef:
      name: mariadb-server-ca
    clientCASecretRef:
      name: mariadb-client-ca
```

## Intermediate CAs

Intermediate CAs are supported by the operator with [some limitations](#limitations). Leaf certificates issued by the intermediate CAs are slightly different, and include the intermediate CA public key as part of the certificate, in the following order: `Leaf certificate -> Intermediate CA`. This is a common practise to easily establish trust in complex PKI setups, where multiple CA are involved. 

Many applications support this `Leaf certificate -> Intermediate CA` structure as a valid leaf certificate, and are able to establish trust with the intermediate CA. Normally, the intermediate CA will not be directly trusted, but used as a path to the root CA, which should be trusted by the application. If not trusted already, you can add the root CA to the [CA bundle](#ca-bundle) by using a [custom trust](#custom-trust).

## Custom trust

You are able to provide a set of CA public keys to be added to the [CA bundle](#ca-bundle) by creating a `Secret` with the following structure:

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: custom-trust
  labels:
    k8s.mariadb.com/watch: ""
data:
  ca.crt:
  -----BEGIN CERTIFICATE-----
  <my-org-root-ca>
  -----END CERTIFICATE-----
  -----BEGIN CERTIFICATE-----
  <root-ca>
  -----END CERTIFICATE-----
```

And referencing it in the `MariaDB` and `MaxScale` resources, for instance:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  ...
  tls:
    enabled: true
    adminCASecretRef:
      name: custom-trust
    adminCertIssuerRef:
      name: my-org-intermediate-ca
      kind: ClusterIssuer
    listenerCASecretRef:
      name: custom-trust
    listenerCertIssuerRef:
      name: intermediate-ca
      kind: ClusterIssuer
```

This is specially useful when issuing certificates with an intermediate CA, see [intermediate CAs](#intermediate-cas) section for further detail.

## Distributing trust

Distributing the [CA bundle](#ca-bundle) to your application namespace is out of the scope of this operator, the bundles will remain in the same namespace as the `MariaDB` and `MaxScale` instances.

If your application is in a different namespace, you can copy the CA bundle to the application namespace. Projects like [trust-manager](https://github.com/cert-manager/trust-manager) can help you to automate this process and continously reconcile bundle changes.

## CA renewal

Depending on the setup, CAs can be managed and renewed by either mariadb-operator or cert-manager. 

When managed by mariadb-operator, CAs have a lifetime of 3 years and marked for renewal after 66% of its lifetime has passed i.e. ~2 years. After being renewed, the operator will trigger an update of the instances to include the new CA in the bundle. 

When managed by cert-manager, the renewal process is fully controlled by cert-manager, but the operator will also update the CA bundle after the CA is renewed.

You may choose any of the available [update strategies](./UPDATES.md) to control the instance update process.

## Certificate renewal

Depending on the setup, certificates can be managed and renewed by mariadb-operator or cert-manager. In either case, certificates have a lifetime of 90 days and marked for renewal after 66% of its lifetime has passed i.e. ~60 days.

When the [certificates are issued by the mariadb-operator](#issue-certificates-with-mariadb-operator), the operator is able to pause a leaf certificate renewal if the CA is being updated at that same moment. This approach ensures a smooth update by avoiding the simultaneous rollout of the new CA and its associated certificates. Rolling them out together could be problematic, as all Pods need to trust the new CA before its issued certificates can be utilized.

When the [certificates are issued by cert-manager](#issue-certificates-with-cert-manager), the renewal process is fully managed by cert-manager, and the operator will not interfere with it. The operator will only update the instances whenever the CA or the certificates get renewed.

You may choose any of the available [update strategies](./UPDATES.md) to control the instance update process.

## Certificate status

To have a high level picture of the certificates status, you can check the `status.tls` field of the `MariaDB` and `MaxScale` resources:

```bash
kubectl get mariadb mariadb-galera -o jsonpath="{.status.tls}" | jq
{
  "caBundle": [
    {
      "issuer": "CN=mariadb-galera-ca",
      "notAfter": "2028-01-20T14:26:50Z",
      "notBefore": "2025-01-20T13:26:50Z",
      "subject": "CN=mariadb-galera-ca"
    }
  ],
  "clientCert": {
    "issuer": "CN=mariadb-galera-ca",
    "notAfter": "2025-04-20T14:26:50Z",
    "notBefore": "2025-01-20T13:26:50Z",
    "subject": "CN=mariadb-galera-client"
  },
  "serverCert": {
    "issuer": "CN=mariadb-galera-ca",
    "notAfter": "2025-04-20T14:26:50Z",
    "notBefore": "2025-01-20T13:26:50Z",
    "subject": "CN=mariadb-galera.default.svc.cluster.local"
  }
}
``` 

```bash
kubectl get maxscale maxscale-galera -o jsonpath="{.status.tls}" | jq
{
  "adminCert": {
    "issuer": "CN=maxscale-galera-ca",
    "notAfter": "2025-04-20T14:33:09Z",
    "notBefore": "2025-01-20T13:33:09Z",
    "subject": "CN=maxscale-galera.default.svc.cluster.local"
  },
  "caBundle": [
    {
      "issuer": "CN=maxscale-galera-ca",
      "notAfter": "2028-01-20T14:33:09Z",
      "notBefore": "2025-01-20T13:33:09Z",
      "subject": "CN=maxscale-galera-ca"
    },
    {
      "issuer": "CN=mariadb-galera-ca",
      "notAfter": "2028-01-20T14:28:46Z",
      "notBefore": "2025-01-20T13:28:46Z",
      "subject": "CN=mariadb-galera-ca"
    }
  ],
  "listenerCert": {
    "issuer": "CN=maxscale-galera-ca",
    "notAfter": "2025-04-20T14:33:09Z",
    "notBefore": "2025-01-20T13:33:09Z",
    "subject": "CN=maxscale-galera.default.svc.cluster.local"
  },
  "serverCert": {
    "issuer": "CN=mariadb-galera-ca",
    "notAfter": "2025-04-20T14:28:46Z",
    "notBefore": "2025-01-20T13:28:46Z",
    "subject": "CN=mariadb-galera-client"
  }
}
``` 

## TLS requirements for `Users`

You are able to declaratively manage access to your `MariaDB` instances by creating [`User` SQL resources](./SQL_RESOURCES.md#user-cr). In particular, when TLS is enabled, you can provide additional requirements for the user when connecting over TLS.

For instance, if you want to require a valid x509 certificate for the user to be able o connect:

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

In order to restrict which subject the user certificate should have and/or require a particular issuer, you may set:

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

When any of these TLS requirements are not met, the user will not be able to connect to the instance.

See [MariaDB docs](https://mariadb.com/kb/en/securing-connections-for-client-and-server/#requiring-tls) and the [API reference](./API_REFERENCE.md) for further detail.

## Secure application connections with TLS

In this guide, we will configure TLS for an application running in the `app` namespace to connect with `MariaDB` and `MaxScale` instances deployed in the `default` namespace. We assume that the following resources are already present in the `default` namespace:  
- [`MariaDB` Galera](../examples/manifests/mariadb_galera_tls.yaml)  
- [`MaxScale` Galera](../examples/manifests/maxscale_galera_tls.yaml)  

The first step is to create a `User` resource and grant the necessary permissions:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: User
metadata:
  name: app
  namespace: app
spec:
  mariaDbRef:
    name: mariadb-galera
    namespace: default
  require:
    issuer: "/CN=mariadb-galera-ca"
    subject: "/CN=mariadb-galera-client"
  host: "%"
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: Grant
metadata:
  name: grant-app
  namespace: app
spec:
  mariaDbRef:
    name: mariadb-galera
    namespace: default
  privileges:
    - "ALL PRIVILEGES"
  database: "*"
  table: "*"
  username: app
  host: "%"
```

The `app` user will be able to connect to the `MariaDB` instance from the `app` namespace by providing a certificate with subject `mariadb-galera-client` and issued by the `mariadb-galera-ca` CA.

With the permissions in place, the next step is to prepare the certificates required for the application to connect:
- **CA Bundle**: The trust bundle for `MariaDB` and `MaxScale` is available as a `Secret` named `<instance-name>-ca-bundle` in the `default` namespace. For more details, refer to the sections on [CA bundle](#ca-bundle) and [distributing trust](#distributing-trust). Additionally, check out the [trust-manager `Bundle` example](../hack/manifests/trust-manager/bundle.yaml), which demonstrates how to copy a bundle to the `app` namespace.  
- **Client Certificate**: `MariaDB` provides a default client certificate stored in a `Secret` named `<mariadb-name>-client-cert` in the `default` namespace. You can either use this `Secret` or generate a new one with the subject `mariadb-galera-client`, issued by the `mariadb-galera-ca` CA. While issuing client certificates for applications falls outside the scope of this operator, you can [test them using `Connection` resources](#test-tls-certificates-with-connections).

In this example, we assume that the following `Secrets` are available in the `app` namespace:  
- `mariadb-bundle`: CA bundle for the `MariaDB` and `MaxScale` instances.  
- `mariadb-galera-client-cert`: Client certificate required to connect to the `MariaDB` instance.  

With these `Secrets` in place, we can proceed to define our application:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: mariadb-client
  namespace: app
spec:
  schedule: "*/1 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: mariadb-client
            image: mariadb:11.4.4
            command:
              - bash
            args:
              - -c
              - >
                mariadb -u app -h mariadb-galera-primary.default.svc.cluster.local
                --ssl-ca=/etc/pki/ca.crt --ssl-cert=/etc/pki/tls.crt
                --ssl-key=/etc/pki/tls.key --ssl-verify-server-cert
                -e "SELECT 'MariaDB connection successful!' AS Status;" -t
            volumeMounts:
            - name: pki
              mountPath: /etc/pki
              readOnly: true
          volumes:
          - name: pki
            projected:
              sources:
              - secret:
                  name: mariadb-bundle
                  items:
                  - key: ca.crt
                    path: ca.crt
              - secret:
                  name: mariadb-galera-client-cert
                  items:
                  - key: tls.crt
                    path: tls.crt
                  - key: tls.key
                    path: tls.key
          restartPolicy: Never
```

The application will connect to the `MariaDB` instance using the `app` user, and will execute a simple query to check the connection status. The `--ssl-ca`, `--ssl-cert`, `--ssl-key` and `--ssl-verify-server-cert` flags are used to provide the CA bundle, client certificate and key, and to verify the server certificate respectively. 

If the connection is successful, the output should be:
```bash
+---------------------------------+
| Status                          |
+---------------------------------+
| MariaDB connection successful!  |
+---------------------------------+
```

You can also point the application to the `MaxScale` instance by updating the host to `maxscale-galera.default.svc.cluster.local`:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: maxscale-client
  namespace: app
spec:
  schedule: "*/1 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: maxscale-client
            image: mariadb:11.4.4
            command:
              - bash
            args:
              - -c
              - >
                mariadb -u app -h maxscale-galera.default.svc.cluster.local
                --ssl-ca=/etc/pki/ca.crt --ssl-cert=/etc/pki/tls.crt
                --ssl-key=/etc/pki/tls.key --ssl-verify-server-cert
                -e "SELECT 'MaxScale connection successful!' AS Status;" -t
            volumeMounts:
            - name: pki
              mountPath: /etc/pki
              readOnly: true
          volumes:
          - name: pki
            projected:
              sources:
              - secret:
                  name: mariadb-bundle
                  items:
                  - key: ca.crt
                    path: ca.crt
              - secret:
                  name: mariadb-galera-client-cert
                  items:
                  - key: tls.crt
                    path: tls.crt
                  - key: tls.key
                    path: tls.key
          restartPolicy: Never
```

If successful, the expected output is:
```bash
+---------------------------------+
| Status                          |
+---------------------------------+
| MaxScale connection successful! |
+---------------------------------+
```

## Test TLS certificates with `Connections`

In order to validate your TLS setup, and to ensure that you TLS certificates are correctly issued and configured, you can use the `Connection` resource to test the connection to both your `MariaDB` and `MaxScale` instances:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection
spec:
  mariaDbRef:
    name: mariadb-galera
  username: mariadb
  passwordSecretKeyRef:
    name: mariadb
    key: password
  tlsClientCertSecretRef:
    name: mariadb-galera-client-cert
  database: mariadb
  healthCheck:
    interval: 30s
```

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Connection
metadata:
  name: connection-maxscale
spec:
  maxScaleRef:
    name: maxscale-galera
  username: mariadb
  passwordSecretKeyRef:
    name: mariadb
    key: password
  tlsClientCertSecretRef:
    name: mariadb-galera-client-cert
  database: mariadb
  healthCheck:
    interval: 30s
```

If successful, the `Connection` resource will be in a `Ready` state, which means that your TLS setup is correctly configured:

```bash
kubectl get connections
NAME                         READY   STATUS    SECRET                AGE
connection                   True    Healthy   connection            2m8s
connection-maxscale          True    Healthy   connection-maxscale   97s
```

This could be specially useful when [providing your own certificates](#provide-your-own-certificates) and issuing certificates for your applications.

## Enabling TLS in existing instances

Follow these steps to migrate existing `MariaDB` Galera and `MaxScale` instances to TLS without downtime:

1. Ensure that `MariaDB` has TLS enabled and not enforced. Set the following options if needed:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  tls:
+   enabled: true
+   required: false
+   galeraSSTEnabled: false
```

By setting these options, the operator will issue and configure certificates for `MariaDB`, but TLS will not be enforced in the connections i.e. both TLS and non-TLS connections will be accepted. TLS enforcement will be optionally configured at the end of the migration process.

This will trigger a rolling upgrade, make sure it finishes successfully before proceeding with the next step. Refer to the [updates documentation](./UPDATES.md) for further information about update strategies.

2. If you are currently using `MaxScale`, it is important to note that, unlike `MariaDB`, it does not support TLS and non-TLS connections simultaneously (see [limitations](#limitations)). For this reason, you must temporarily point your applications to `MariaDB` during the migration process. You can achieve this by configuring your application to use the [`MariaDB Services`](./HA.md#kubernetes-services). At the end of the `MariaDB` migration process, the `MaxScale` instance will need to be recreated in order to use TLS, and then you will be able to point your application back to `MaxScale`. Ensure that all applications are pointing to `MariaDB` before moving on to the next step.

3. `MariaDB` is now accepting TLS connections. The next step is [migrating your applications to use TLS](#secure-application-connections-with-tls) by pointing them to `MariaDB` securely. Ensure that all applications are connecting to `MariaDB` via TLS before proceeding to the next step.

4. If you are currently using `MaxScale`, and you are planning to connect via TLS through it, you should now delete your `MaxScale` instance. If needed, keep a copy of the `MaxScale` manifest, as we will need to recreate it with TLS enabled in further steps: 

```bash
kubectl get mxs maxscale-galera -o yaml > maxscale-galera.yaml
kubectl delete mxs maxscale-galera
```
It is very important that you wait until your old `MaxScale` instance is fully terminated to make sure that the old configuration is cleaned up by the operator.

5. For enhanced security, it is recommended to enforce TLS in all `MariaDB` connections by setting the following option. This will trigger a rolling upgrade, make sure it finishes successfully before proceeding with the next step:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  tls:
+   required: true
```

6. For improved security, you can optionally configure TLS for Galera SSTs by following the steps below:

  - Run [this migration script](../hack/migrate_galera_sst_ssl.sh):
```bash
 ./hack/migrate_galera_sst_ssl.sh <mariadb-name> # e.g. ./migrate_galera_sst_ssl.sh mariadb-galera
```

  - Set the following option to enable TLS for Galera SSTs:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  tls:
+   galeraSSTEnabled: true
```
This will trigger a rolling upgrade, make sure it finishes successfully before proceeding with the next step

7. As mentioned in step `4.`, recreate your `MaxScale` instance with `tls.enabled=true` if needed:
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
+ tls:
+   enabled: true
```

8. `MaxScale` is now accepting TLS connections. Next, you need to [migrate your applications to use TLS](#secure-application-connections-with-tls) by pointing them back to `MaxScale` securely. You have done this previously for `MariaDB`, you just need to update your application configuration to use the [`MaxScale Service`](./MAXSCALE.md#kubernetes-services) and its CA bundle.


## Limitations

### Galera and intermediate CAs

Leaf certificates issued by [intermediate CAs](#intermediate-cas) are not supported by Galera, see [MDEV-35812](https://jira.mariadb.org/browse/MDEV-35812). This implies that a root CA must be used to issue the `MariaDB` certificates.

This doesn't affect `MaxScale`, as it is able to establish trust with intermediate CAs, and therefore you can still issue your application facing certificates (MaxScale listeners) with an intermediate CA, giving you more flexibility in your PKI setup. You can find a practical [example here](./../examples/manifests/maxscale_galera_tls_cert_manager_intermediate_ca.yaml).


### MaxScale

- Unlike `MariaDB`, TLS and non-TLS connections on the same are not supported simultanously.
- TLS encryption must be enabled for listeners when they are created. For servers, the TLS can be enabled after creation but it cannot be disabled or altered.

Refer to the [MaxScale documentation ](https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#tlsssl-encryption)for further detail.