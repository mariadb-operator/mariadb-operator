## Galera TLS migration

This runbook allows you to enable TLS on an existing Galera instance without downtime:

- Add the following fields to your existing Galera instance:

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
```
- Once the rolling update has finished, remove the `socket.dynamic` provider option

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
```

- Trigger a rolling update if needed