# Replication

The operator supports provisioning and operating MariaDB clusters with semi-synchronous replication as a high availability topology. In the following sections we will cover how to manage the full lifecycle of a semi-synchronous replication cluster. Please refer to the [MariaDB documentation for more details about replication.](https://mariadb.com/docs/server/ha-and-performance/standard-replication)

## Table of contents
<!-- toc -->
<!-- /toc -->

## Provisioning

In order to provision a replication cluster, you need to configure a number of `replicas` greater than `1` and set the `replication.enabled=true` in the `MariaDB` CR:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replicas: 3
  replication:
    enabled: true
```

After applying the previous CR, the operator will provision a replication cluster with one primary and two replicas. The operator will take care of setting up replication, configuring the replication user and monitoring the replication status:

```bash
kubectl get pods
NAME                                    READY   STATUS    RESTARTS   AGE
mariadb-repl-0                          2/2     Running   0          2d19h
mariadb-repl-1                          2/2     Running   0          2d19h
mariadb-repl-2                          2/2     Running   0          2d19h
mariadb-repl-metrics-56865fff65-t72kc   1/1     Running   0          2d20h

kubectl get mariadb
NAME           READY   STATUS    PRIMARY          UPDATES                    AGE
mariadb-repl   True    Running   mariadb-repl-0   ReplicasFirstPrimaryLast   2d20h
```

As you can see, the primary can be identified in the `PRIMARY` column of the `kubectl get mariadb` output. You may also check the current replication status by checking the `MariaDB` CR status:

```bash
kubectl get mariadb mariadb-repl -o jsonpath="{.status.replication}" | jq
{
  "replicas": {
    "mariadb-repl-0": {
      "gtidCurrentPos": "0-10-24",
      "gtidIOPos": "0-10-24",
      "lastErrorTransitionTime": "2025-10-17T14:30:10Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true
    },
    "mariadb-repl-2": {
      "gtidCurrentPos": "0-10-24",
      "gtidIOPos": "0-10-24",
      "lastErrorTransitionTime": "2025-10-17T14:30:10Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true
    }
  },
  "roles": {
    "mariadb-repl-0": "Primary",
    "mariadb-repl-1": "Replica",
    "mariadb-repl-2": "Replica"
  }
}
```

The operator continiously monitors the replication status via [`SHOW SLAVE STATUS`](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/show/show-replica-status), taking it into account for internal operations and updating the CR status accordingly.

## Configuration

The replication settings can be customized under the `replication` section of the MariaDB CR. The following options are available:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replicas: 3
  replication:
    enabled: true
    gtidStrictMode: true
    waitPoint: AfterCommit
    ackTimeout: 10s
    syncBinlog: 1
```

- `gtidStrictMode`: Enables GTID strict mode on the primary and replicas. It is recommended and enabled by default.
- `waitPoint`: Determines whether the transaction should wait for an ACK before committing to the storage engine.
- `ackTimeout`
- `syncBinlog`:


Additional system variables may be configured via the `myCnf` configuration field. Refer to the [configuration documentation for more details](./configuration.md#mycnf)


## Replica configuration

## Probes

Data plane. Probes documentation

#### Liveness probe

#### Readiness probe

## Replica lag

maxLagSeconds

## Backing up and restoring

GTID
VolumeSnapshot

## Primary switchover

How to trigger it. Steps

## Primary failover

How to configure it. Steps.

## Updates

Steps. Include MaxScale switchover.

## Scaling out

## Replica recovery

## Troubleshooting

#### Current status and events

#### Common errors