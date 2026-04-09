# Security

## Root Password

The root password for a `MariaDB` resource can be specified in a Secret, like so:

```yaml
apiVersion: enterprise.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  # [...]
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
  # [...]
```

By default, `rootPasswordSecretKeyRef` are optional and defaulted by the operator, resulting in random password generation if not provided:

```yaml
apiVersion: enterprise.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  # [...]
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
    generate: true
  # [...]
```

You may choose to explicitly provide a `Secret` reference via `rootPasswordSecretKeyRef` and opt-out from random password generation by either not providing the `generate` field or setting it to `false`:

```yaml
apiVersion: enterprise.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  # [...]
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
    generate: false
  # [...]
```

This way, we are telling the operator that we are expecting a `Secret` to be available eventually, enabling the use of GitOps tools to seed the password:

* [sealed-secrets](https://github.com/bitnami-labs/sealed-secrets): The `Secret` is reconciled from a `SealedSecret`, which is decrypted by the sealed-secrets controller.
* [external-secrets](https://github.com/external-secrets/external-secrets): The `Secret` is reconciled fom an `ExternalSecret`, which is read by the external-secrets controller from an external secrets source (Vault, AWS Secrets Manager ...).

### Password Rotation

> [!IMPORTANT] **Warning**
> It is highly recommended to enable SSL connections to the MariaDB server. If SSL is not enabled, the root password will be transmitted in plain text over the network during the rotation process.

When the value in the `Secret` referenced by `spec.rootPasswordSecretKeyRef` is updated, the operator will perform the following actions to rotate the root password:
1. It connects to the `MariaDB` server using the previous (old) root password.
2. It issues `ALTER USER` commands to update the password for both `root@localhost` and `root@%` to the new value.
3. Upon successful password change in the database, it updates its internal state to reflect that the new password is in place.
4. If the password update fails, it will attempt to revert the password back to the previous value to ensure the system remains in a consistent state.

The operator will only attempt a password rotation when the `MariaDB` cluster is in a stable and healthy state. It will wait for any ongoing operations such as initialization, updates, backups, or scaling to complete before changing the password.

#### Data Plane Updates

The operator also ensures that the root password is propagated to other components that might need it:
- **Agent**: The operator updates the `MARIADB_ROOT_PASSWORD` environment variable in the agent sidecar containers. This allows the liveness and readiness probes to connect to the database.
- **Galera**: If Galera is enabled, the operator updates the `wsrep_sst_auth` credentials used for State Snapshot Transfers (SST).

#### Updating the password

To update the root password, you just need to update the `Secret` referenced by `rootPasswordSecretKeyRef`. For example, using `kubectl`:
```bash
kubectl patch secret mariadb -p '{"data":{"root-password":"<new-password-base64>"}}'
```
Make sure the new password is base64 encoded. The operator will then automatically trigger the password rotation process.