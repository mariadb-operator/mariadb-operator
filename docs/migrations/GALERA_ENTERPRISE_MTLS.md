## Galera Enterprise mTLS migration

> [!IMPORTANT]  
> This runbook applies to MariaDB Enterprise server version >= 10.6. Make sure you are on this version before proceeding.

This runbook allows you to enable mTLS on an existing Galera Enterprise instance without downtime:

- Add the following fields to your existing Galera Enterprise >= 10.6 instance:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  galera:
    enabled: true
+   providerOptions:
+     socket.dynamic: 'true'

  tls:
+   enabled: true

+   galeraServerSSLMode: PROVIDER
+   galeraClientSSLMode: DISABLED
```
- __[Trigger a rolling update](../UPDATES.md)__ if needed. If you use the `ReplicasFirstPrimaryLast` strategy, it will be automatically triggered by the operator
- Once the rolling update has finished, update `galeraServerSSLMode=SERVER_X509`
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  tls:
+   galeraServerSSLMode: SERVER_X509
```
- Trigger a rolling update if needed
- Once the rolling update has finished, run the following script to enable `ssl_mode` on the client side:
```bash
 ./hack/migrate_galera_ssl_mode.sh <mariadb-galera-name> VERIFY_IDENTITY
```
- Update `galeraClientSSLMode=VERIFY_IDENTITY` and remove the `socket.dynamic` provider option
```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  galera:
    enabled: true
-   providerOptions:
-     socket.dynamic: 'true'

  tls:
+   galeraClientSSLMode: VERIFY_IDENTITY
```
- Trigger a rolling update if needed

Refer to the [MariaDB Enterprise Cluster Security docs](https://mariadb.com/docs/server/security/galera/) for further detail.