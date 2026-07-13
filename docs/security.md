# Security

## Table of contents
<!-- toc -->
- [Root Password](#root-password)
- [Custom Container Isolation](#custom-container-isolation)
<!-- /toc -->

## Root Password

The root password for a `MariaDB` resource can be specified in a Secret, like so:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
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
apiVersion: k8s.mariadb.com/v1alpha1
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
apiVersion: k8s.mariadb.com/v1alpha1
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

- [sealed-secrets](https://github.com/bitnami/sealed-secrets): The `Secret` is reconciled from a `SealedSecret`, which is decrypted by the sealed-secrets controller.
- [external-secrets](https://github.com/external-secrets/external-secrets): The `Secret` is reconciled fom an `ExternalSecret`, which is read by the external-secrets controller from an external secrets source (Vault, AWS Secrets Manager ...).

### Password Rotation

> [!IMPORTANT]
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

## Custom Container Isolation

Additional `initContainers` and `sidecarContainers` historically inherit the complete MariaDB environment and volume-mount set. This remains the default when `inheritance` or `inheritance.policy` is omitted, so existing resources keep their expanded `PodSpec` without changes.

Each additional container can independently select one of these policies:

| Policy | Behavior |
| --- | --- |
| `Legacy` | Inherit every MariaDB environment variable and volume mount, then append the container's authored values. This is also the behavior when the policy is omitted. |
| `Isolated` | Start with empty environment-variable and volume-mount lists. Only values authored directly on that container are included. |
| `Selected` | Inherit only the explicitly named semantic groups, in a deterministic order, then append the container's authored values. |

The environment-variable groups are:

| Group | Contents |
| --- | --- |
| `Runtime` | Non-secret MariaDB and Pod runtime metadata. |
| `TLS` | TLS paths and settings. Available only when TLS is enabled. |
| `Replication` | Replication settings. Available only when replication is enabled. |
| `RootPassword` | The root-password Secret reference or empty-password setting. Select this group only when the container requires root access. |
| `User` | Environment variables authored in `spec.env`. |

The volume-mount groups are:

| Group | Contents |
| --- | --- |
| `Config` | Generated MariaDB configuration. |
| `TLS` | TLS certificates and keys. |
| `Storage` | The MariaDB data directory. |
| `Replication` | Generated replication configuration. |
| `AgentAuth` | Data-plane agent authentication material. |
| `ServiceAccount` | The projected data-plane service-account token. |
| `Galera` | Generated Galera configuration. |
| `PointInTimeRecovery` | Point-in-time recovery storage TLS CA mounts. |
| `User` | Volume mounts authored in `spec.volumeMounts`. |

Unknown, duplicate, unavailable, or contradictory selections are rejected. `Isolated` and `Selected` also reject duplicate environment-variable names and duplicate mount paths instead of relying on list ordering. New groups are never implicitly included by `Selected`.

The following sidecar receives runtime metadata but no root password, data directory, configuration, TLS keys, or service-account token:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  sidecarContainers:
    - name: observer
      image: busybox:1.36
      command: ["sh", "-c", "sleep infinity"]
      inheritance:
        policy: Selected
        env:
          - Runtime
      securityContext:
        runAsNonRoot: true
        allowPrivilegeEscalation: false
        privileged: false
        readOnlyRootFilesystem: true
        capabilities:
          drop:
            - ALL
        seccompProfile:
          type: RuntimeDefault
  storage:
    size: 1Gi
```

Apply the updated CRDs before upgrading the controller. Downgrading to a version whose CRDs do not contain `inheritance` or the custom-container `securityContext` is unsupported while resources use those fields.
