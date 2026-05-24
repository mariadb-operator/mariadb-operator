# Maintenance

The operator provides a maintenance mode that allows you to safely perform maintenance operations on a MariaDB cluster. When enabled, maintenance mode gives you fine-grained control over how the database behaves during maintenance windows, including blocking new connections, draining existing connections, and setting the database to read-only mode.

Maintenance mode is designed to work with any MariaDB topology and is particularly useful for:

- **Cluster switchover**: Preventing writes to the primary cluster before switching to a replica cluster in a [multi-cluster](./multi-cluster.md) setup. You can ensure that no writes are lost during the switchover process, allowing the replicas to catch up to the primary.
- **Debugging**: Isolating the database from application traffic while investigating issues.

> [!IMPORTANT]
> Maintenance mode is different from [suspending reconciliation](./scheduling.md#suspending-reconciliation). While suspending reconciliation stops the operator from managing the resource entirely, maintenance mode allows the operator to continue running while controlling how the database behaves.

## Table of contents
<!-- toc -->
- [Enabling maintenance mode](#enabling-maintenance-mode)
- [Cordon mode](#cordon-mode)
- [Drain connections](#drain-connections)
- [Read-only mode](#read-only-mode)
- [Composing maintenance modes](#composing-maintenance-modes)
- [Readiness during maintenance](#readiness-during-maintenance)
- [Disabling maintenance mode](#disabling-maintenance-mode)
- [MaxScale maintenance mode](#maxscale-maintenance-mode)
<!-- /toc -->

## Enabling maintenance mode

To enable maintenance mode, set `spec.maintenance.enabled: true` in the `MariaDB` CR:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-eu-south 
spec:
  maintenance:
    enabled: true
```

This will result in the following status:

```bash
kubectl get mariadb
NAME                 READY   STATUS        PRIMARY                UPDATES                    AGE
mariadb-eu-south     True    Maintenance   mariadb-eu-south-0     ReplicasFirstPrimaryLast   91m
```

The following subsections describe the maintenance mode in detail.

## Cordon mode

Cordon mode blocks all new connections to the MariaDB cluster. When enabled, the operator modifies the Kubernetes service to remove the MariaDB Pods from the service endpoints, effectively preventing new connections from being established.

Existing connections that are already established will continue to work, but any new connection attempts will fail. This is useful when you want to prevent new application traffic from reaching the database while allowing existing connections to complete their work.

To enable cordon mode:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-eu-south
spec:
  maintenance:
    enabled: true
    cordon: true
```

This will result in the following status:

```bash
kubectl get mariadb
NAME                 READY   STATUS        PRIMARY                UPDATES                    AGE
mariadb-eu-south     True     Cordoned     mariadb-eu-south-0     ReplicasFirstPrimaryLast   91m
```

> [!NOTE]
> Cordon mode only affects new connections through Kubernetes services. Direct Pod connections and already established connections are not affected.

## Drain connections

Drain connections mode gracefully terminates long-running connections after a grace period. This allows in-flight queries to complete while preventing new long-running queries from starting.

The operator evaluates all active connections and terminates those that have been running longer than the specified grace period (`spec.maintenance.drainGracePeriodSeconds`). Connections that are still within the grace period are left untouched, giving them time to complete.

The following connection types are considered safe to terminate:
- Client connections (user queries)
- Prepared statements

The following connection types are **never** terminated:
- Replication connections
- System connections

To enable drain connections mode with a custom grace period:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  maintenance:
    enabled: true
    drainConnections: true
    drainGracePeriodSeconds: 30
```

> [!TIP]
> The default grace period is 30 seconds. Adjust this value based on your expected query durations. A longer grace period gives applications more time to complete their work, but may delay the maintenance operation.

## Read-only mode

Read-only mode sets the database to read-only, preventing any write operations (INSERT, UPDATE, DELETE, CREATE, DROP, ALTER, etc.). Read operations (SELECT) continue to work normally.

This is useful when you need to prevent any data modifications, and therefore, allowing the replicas to sync with the primary. 

To enable read-only mode:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  maintenance:
    enabled: true
    readOnly: true
```

> [!NOTE]
> When maintenance mode is enabled without `readOnly`, the operator still sets replicas to read-only in a replication topology.

## Composing maintenance modes

You can combine multiple maintenance modes to achieve the desired behavior. The following combinations are commonly used:

### Full maintenance

This combination provides the most comprehensive maintenance mode, blocking new connections, draining existing connections, and setting the database to read-only:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  maintenance:
    enabled: true
    cordon: true
    drainConnections: true
    drainGracePeriodSeconds: 30
    readOnly: true
```

### Read-only only

This combination only sets the database to read-only, allowing new connections and existing queries to continue:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  maintenance:
    enabled: true
    readOnly: true
```

### Drain only

This combination only drains long-running connections, allowing new connections and short queries to continue:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  maintenance:
    enabled: true
    drainConnections: true
    drainGracePeriodSeconds: 60
```

## Readiness during maintenance

When maintenance mode is enabled, the MariaDB resource's readiness state changes to reflect the maintenance status:

| Condition | Reason | Message |
|-----------|--------|---------|
| `Ready=False` | `Cordoned` | Cordoned |
| `Ready=True` | `Maintenance` | Maintenance |

When `cordon` is enabled, the resource is marked as not ready (`Ready=False`) with the reason `Cordoned`. This prevents Kubernetes from routing traffic to the database.

When cordon is disabled but maintenance mode is enabled, the resource is marked as ready (`Ready=True`) with the reason `Maintenance`. This indicates that the database is in maintenance mode but still accepting connections.

## Disabling maintenance mode

To disable maintenance mode, set `spec.maintenance.enabled: false`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  maintenance:
    enabled: false
```

When maintenance mode is disabled, the operator will:
1. Disable read-only mode on all Pods (if it was enabled).
2. Re-add the Pods to the service endpoints (if cordon was enabled).

## MaxScale maintenance mode

The `MaxScale` CR also supports maintenance mode with a similar mechanism. When enabled, it cordons the Kubernetes service by modifying the service selector labels, effectively removing MaxScale Pods from the endpoints and blocking new connections.

Unlike MariaDB, MaxScale maintenance mode only provides cordon functionality. It does not support draining connections or read-only mode, as MaxScale acts as a proxy rather than a database.

To enable maintenance mode on `MaxScale`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  maintenance:
    enabled: true
    cordon: true
```

This behaves similarly to MariaDB's cordon mode: existing connections through the service are not immediately terminated, but new connection attempts will fail as the Pods are removed from the service endpoints.

> [!NOTE]
> MaxScale also supports putting individual backend MariaDB servers in maintenance mode. See the [Server maintenance](./maxscale.md#server-maintenance) section for details.