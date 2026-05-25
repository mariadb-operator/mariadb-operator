# Multi-cluster

The multi-cluster feature enables high availability by replicating data between multiple MariaDB clusters. It builds on top of either [replication](./replication.md) or [Galera](./galera.md) clusters, creating a topology where one cluster acts as the primary and the others as replicas, with each cluster maintaining its own internal HA topology.

A multi-cluster setup can be deployed in two ways:

- **Across multiple Kubernetes clusters**: Each Kubernetes cluster runs a MariaDB cluster with its own HA mechanism (replication or Galera). The clusters are connected via remote replication, forming a hierarchy where the primary cluster receives all write operations and the replica clusters replicate data from it. This provides both intra-cluster HA (within each cluster) and inter-cluster HA (across Kubernetes clusters).
- **Within a single Kubernetes cluster**: A single Kubernetes cluster can host multiple MariaDB clusters with local replication configured between them. This is useful for blue-green deployments, where one cluster serves traffic while the other is updated in the background, enabling zero-downtime upgrades without data loss.

Please refer to the [replication](./replication.md) and [Galera](./galera.md) documentation for more details about the underlying HA topologies.

## Table of contents
<!-- toc -->
- [Introduction](#introduction)
- [Use cases](#use-cases)
- [Architecture](#architecture)
- [Provisioning](#provisioning)
  - [Prerequisites](#prerequisites)
  - [Provisioning process](#provisioning-process)
  - [Scenarios](#scenarios)
- [Cluster switchover](#cluster-switchover)
- [Limitations](#limitations)
- [Troubleshooting](#troubleshooting)
<!-- /toc -->

## Introduction

The multi-cluster feature extends the MariaDB operator's high availability capabilities by connecting multiple MariaDB clusters via replication. Please refer to the [architecture diagram](#architecture) for a visual representation.

Each MariaDB cluster runs its own HA topology (replication or Galera), and the clusters are connected via a dedicated replication connection. The primary cluster's primary Pod acts as the source of truth, and the replica cluster's primary Pod (called the "primary replica") replicates from it.

The operator handles the full lifecycle of this topology, including:
- Provisioning the primary and replica MariaDB clusters
- Taking the physical backup of the primary cluster
- Bootstrapping the replica cluster from the physical backup
- Configuring the replication connection between clusters
- Performing cluster-level switchover when needed

## Use cases

### Multi-region deployments

Deploy MariaDB clusters across different geographic regions for disaster recovery and reduced latency. The primary cluster in one region handles all write operations, while replica clusters in other regions provide read scalability and regional failover capability.

### Blue-green deployments

Maintain two identical cluster topologies (blue and green) and switch between them for zero-downtime deployments. While one cluster serves traffic, the other can be updated in the background. This use case does not require multiple Kubernetes clusters — a single Kubernetes cluster can host multiple MariaDB clusters with local replication configured between them, enabling blue-green upgrades without downtime or data loss.

### Active-passive disaster recovery

Run a primary cluster in one region and a passive replica cluster in another region. In case of a regional outage, switch traffic to the replica cluster, which can then become the new primary.

### Data locality

Place replica clusters closer to your application instances to reduce network latency for read operations, while keeping the primary cluster in a central location.

## Architecture

![Multi-cluster architecture](./assets/multi-cluster.png)

The multi-cluster architecture consists of the following components:

- **MariaDB Operator**: Runs on each Kubernetes cluster, responsible for provisioning and managing the `MariaDB` cluster, configuring the internal HA topology (replication or Galera), setting up multi-cluster replication connections, and monitoring replication status.
- **Primary Cluster**: The MariaDB cluster where writes are performed. Its primary Pod is the source of truth for write operations: both local replicas and remote replicas replicate from it.
- **Replica Cluster**: The MariaDB cluster that replicates data from the primary cluster. Its primary replica Pod replicates from the primary cluster's primary Pod and acts as the source of truth for the replica cluster's internal topology. This cluster must only be used for read operations or as disaster recovery standby.
- **MaxScale Service**: An internal Kubernetes Service that routes traffic to `MaxScale` Pods within a cluster. When `MaxScale` is used, the primary replica connects to this service instead of the `MariaDB` Kubernetes service.
- **LoadBalancer**: An external load balancer managed by the user (e.g., cloud provider LB or Metallb in bare metal) that exposes the primary cluster to applications. This load balancer must be manually updated to point to the new primary cluster after a cluster switchover. When deploying within a single Kubernetes cluster (e.g., blue-green), a Kubernetes Service can be used instead of a load balancer.

## Provisioning

The provisioning process involves deploying two MariaDB clusters (primary and replica), each with its own HA topology. The replica cluster bootstraps from a physical backup of the primary cluster.

### Prerequisites

Before provisioning a multi-cluster setup, ensure the following generic prerequisites are met:

- **Shared secrets**: The same root password secret must be available in all MariaDB clusters.
- **TLS certificates**: A shared CA certificate for TLS between clusters.
- **Network connectivity**: Connectivity must be available between all MariaDB clusters.

For multi-region HA deployments, additional requirements apply:

- **Multiple Kubernetes clusters**: At least two Kubernetes clusters with network connectivity between them.
- **LoadBalancer**: An externally managed `LoadBalancer` that exposes the primary cluster for replication connections. It must be manually updated to point to the new primary cluster after a cluster switchover.
- **S3-compatible storage**: An S3-compatible bucket accessible from multiple regions for storing physical backups used for bootstrapping the replica cluster.

### Provisioning process

The provisioning process consists of the following steps:

#### Step 1: Deploy primary cluster

Deploy the primary cluster in the first Kubernetes cluster (eu-south). This cluster will serve as the source of all write operations:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-eu-south
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: password
  storage:
    size: 1Gi
  replicas: 2
  replication:
    enabled: true
    gtidDomainId: 0
    serverIdStartIndex: 10
    # Prevent timeouts from replica ACKs in cross-regional setups.
    semiSyncEnabled: false
    replica:
      replPasswordSecretKeyRef:
        name: mariadb
        key: password
      bootstrapFrom:
        physicalBackupTemplateRef:
          name: physicalbackup-eu-south
      recovery:
        enabled: true
        errorDurationThreshold: 30s
  primaryService:
    type: LoadBalancer
    metadata:
      annotations:
        metallb.io/loadBalancerIPs: 172.18.1.10
  tls:
    enabled: true
    required: true
    serverCASecretRef:
      name: mariadb-server-ca
    serverCertAdditionalNames:
      - 172.18.1.10
    clientCASecretRef:
      name: mariadb-server-ca
  multiCluster:
    enabled: true
    primary: mariadb-eu-south
    members:
      - name: mariadb-eu-south
        externalMariaDbRef:
          name: mariadb-eu-south
      - name: mariadb-eu-central
        externalMariaDbRef:
          name: mariadb-eu-central
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: mariadb-eu-south
spec:
  host: mariadb-eu-south-primary.default.svc.cluster.local
  port: 3306
  username: root
  passwordSecretKeyRef:
    name: mariadb
    key: password
  tls:
    enabled: true
    serverCASecretRef:
      name: mariadb-server-ca
    clientCASecretRef:
      name: mariadb-server-ca
```

Key fields:
- `spec.multiCluster.enabled`: Enables the multi-cluster topology.
- `spec.multiCluster.primary`: The name of the primary cluster member. This must be the name of the current cluster.
- `spec.multiCluster.members`: A list of all clusters in the multi-cluster topology, each with its `ExternalMariaDB` reference.
- `spec.replication.gtidDomainId`: The GTID domain ID for this cluster. The primary cluster uses `0`, and replica clusters use different values (e.g., `1`, `2`, etc.) to prevent GTID conflicts. Refer to [MariaDB docs](https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#gtid_domain_id) for additional documentation.
- `spec.replication.semiSyncEnabled`: Set to `false` for cross-regional setups to avoid ACK timeouts.
- `spec.primaryService`: The service used to expose the primary cluster's primary Pod for replication connections from replica clusters.

Verify the primary cluster is running:

```bash
kubectl get mariadb mariadb-eu-south -o jsonpath="{.status}" | jq '{conditions: .conditions, currentPrimary: .currentPrimary, currentMultiClusterPrimary: .currentMultiClusterPrimary}'
{
  "conditions": [
    {
      "lastTransitionTime": "2026-05-25T18:09:50Z",
      "message": "Running",
      "reason": "StatefulSetReady",
      "status": "True",
      "type": "Ready"
    },
    # [...]
  ],
  "currentPrimary": "mariadb-eu-south-0",
  "currentMultiClusterPrimary": "mariadb-eu-south"
}
```

#### Step 2: Create PhysicalBackup

The replica cluster bootstraps from a physical backup of the primary cluster. Create a `PhysicalBackup` resource that the operator will use to take a full backup of the primary cluster:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: PhysicalBackup
metadata:
  name: physicalbackup-eu-south
spec:
  mariaDbRef:
    name: mariadb-eu-south
  schedule:
    cron: "0 * * * *"
    immediate: true
  target: PreferReplica
  compression: bzip2
  storage:
    s3:
      bucket: multi-cluster
      prefix: eu-south
      endpoint: minio.minio.svc.cluster.local:9000
      region: us-east-1
      accessKeyIdSecretKeyRef:
        name: minio
        key: access-key-id
      secretAccessKeySecretKeyRef:
        name: minio
        key: secret-access-key
      tls:
        enabled: true
        caSecretKeyRef:
          name: minio-ca
          key: ca.crt
```

This `PhysicalBackup` is applied to the **primary cluster** (eu-south). The operator will take a full physical backup and store it in the S3 bucket. This backup will be used to bootstrap the replica cluster.

Verify the backup is complete:

```bash
kubectl get physicalbackup physicalbackup-eu-south -o jsonpath="{.status}" | jq
{
  "conditions": [
    {
      "lastTransitionTime": "2026-05-25T18:10:10Z",
      "message": "Success",
      "reason": "JobComplete",
      "status": "True",
      "type": "Complete"
    }
  ],
  "lastScheduleCheckTime": "2026-05-25T18:10:01Z",
  "lastScheduleTime": "2026-05-25T18:10:01Z"
}
```

#### Step 3: Deploy replica cluster

Deploy the replica cluster in the second Kubernetes cluster (eu-central). This cluster will replicate data from the primary cluster:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-eu-central
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: password
  storage:
    size: 1Gi
  replicas: 2
  bootstrapFrom:
    s3:
      bucket: multi-cluster
      prefix: eu-south
      endpoint: minio.minio.svc.cluster.local:9000
      region: us-east-1
      accessKeyIdSecretKeyRef:
        name: minio
        key: access-key-id
      secretAccessKeySecretKeyRef:
        name: minio
        key: secret-access-key
      tls:
        enabled: true
        caSecretKeyRef:
          name: minio-ca
          key: ca.crt
    backupContentType: Physical
  replication:
    enabled: true
    gtidDomainId: 1
    serverIdStartIndex: 20
    # Prevent timeouts from replica ACKs in cross-regional setups.
    semiSyncEnabled: false
    replica:
      replPasswordSecretKeyRef:
        name: mariadb
        key: password
  primaryService:
    type: LoadBalancer
    metadata:
      annotations:
        metallb.io/loadBalancerIPs: 172.18.1.15
  tls:
    enabled: true
    required: true
    serverCASecretRef:
      name: mariadb-server-ca
    serverCertAdditionalNames:
      - 172.18.1.15
    clientCASecretRef:
      name: mariadb-server-ca
  multiCluster:
    enabled: true
    primary: mariadb-eu-south
    members:
      - name: mariadb-eu-south
        externalMariaDbRef:
          name: mariadb-eu-south
      - name: mariadb-eu-central
        externalMariaDbRef:
          name: mariadb-eu-central
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: mariadb-eu-central
spec:
  host: mariadb-eu-central-primary.default.svc.cluster.local
  port: 3306
  username: root
  passwordSecretKeyRef:
    name: mariadb
    key: password
  tls:
    enabled: true
    serverCASecretRef:
      name: mariadb-server-ca
    clientCASecretRef:
      name: mariadb-server-ca
```

Key differences from the primary cluster:
- `spec.bootstrapFrom`: Points to the S3 bucket where the primary cluster's backups are stored. This is used to bootstrap the replica cluster with the latest data.
- `spec.replication.gtidDomainId`: Set to a different value (`1`) than the primary cluster (`0`). Refer to [MariaDB docs](https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#gtid_domain_id) for additional documentation.
- `spec.replication.serverIdStartIndex`: Set to a different value (`20`) than the primary cluster (`10`) to avoid server ID conflicts. Refer to [MariaDB docs](https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables#server_id) for additional documentation.

When the replica cluster is deployed, the operator will automatically:

1. Download the physical backup from the S3 bucket.
2. Restore the backup to the replica cluster's Pods.
3. Configure the internal replication topology (primary + replicas within the cluster).
4. Configure the multi-cluster replication connection (primary replica -> primary cluster).

Verify the bootstrap is complete by checking the `BackupRestored` and `ReplicationConfigured` conditions in the replica cluster status:

```bash
kubectl get mariadb mariadb-eu-central -o jsonpath="{.status}" | jq '{conditions: .conditions, currentPrimary: .currentPrimary, currentMultiClusterPrimary: .currentMultiClusterPrimary}'
{
  "conditions": [
    {
      "lastTransitionTime": "2026-05-25T18:11:14Z",
      "message": "Running",
      "reason": "StatefulSetReady",
      "status": "True",
      "type": "Ready"
    },
    {
      "lastTransitionTime": "2026-05-25T18:10:55Z",
      "message": "Restored physical backup",
      "reason": "RestorePhysicalBackup",
      "status": "True",
      "type": "BackupRestored"
    },
    {
      "lastTransitionTime": "2026-05-25T18:10:55Z",
      "message": "Replication configured",
      "reason": "ReplicationConfigured",
      "status": "True",
      "type": "ReplicationConfigured"
    }
  ],
  "currentPrimary": "mariadb-eu-central-0",
  "currentMultiClusterPrimary": "mariadb-eu-south"
}
```

Check the replication status to verify the replica cluster is connected to the primary:

```bash
kubectl get mariadb mariadb-eu-central -o jsonpath="{.status.replication}" | jq
{
  "replicas": {
    "mariadb-eu-central-0": {
      "gtidCurrentPos": "0-10-4,1-20-5",
      "gtidIOPos": "0-10-4",
      "lastErrorTransitionTime": "2026-05-25T18:10:55Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true,
      "usingGtid": "Slave_Pos"
    },
    "mariadb-eu-central-1": {
      "gtidCurrentPos": "0-10-4,1-20-5",
      "gtidIOPos": "1-20-5,0-10-4",
      "lastErrorTransitionTime": "2026-05-25T18:10:55Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true,
      "usingGtid": "Slave_Pos"
    }
  },
  "roles": {
    "mariadb-eu-central-0": "PrimaryReplica",
    "mariadb-eu-central-1": "Replica"
  }
}
```

The `PrimaryReplica` Pod (`mariadb-eu-central-0`) replicates from the primary cluster's primary Pod. The `gtidCurrentPos` shows both domain `0` (replicated from the primary cluster) and domain `1` (its own cluster's transactions).

### Scenarios

The following sections describe the different scenarios supported by the multi-cluster feature. All scenarios are available in the [examples catalog](https://github.com/mariadb-operator/mariadb-operator/tree/main/examples/manifests/multi-cluster).

#### Replication

The simplest multi-cluster scenario uses replication as the intra-cluster HA mechanism. Each MariaDB cluster runs a replication topology, and the clusters are connected via replication.

```yaml
# Primary cluster (eu-south)
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-eu-south
spec:
  # [...]
  replicas: 3
  replication:
    enabled: true
    gtidDomainId: 0
  multiCluster:
    enabled: true
    primary: mariadb-eu-south
    members:
      - name: mariadb-eu-south
        externalMariaDbRef:
          name: mariadb-eu-south
      - name: mariadb-eu-central
        externalMariaDbRef:
          name: mariadb-eu-central
  # [...]
```

See the [replication example](https://github.com/mariadb-operator/mariadb-operator/tree/main/examples/manifests/multi-cluster/replication) for complete manifests.

#### Replication with MaxScale

When using MaxScale, the replica cluster's primary replica connects to MaxScale instead of directly to the primary cluster's Pods. This provides connection pooling, query routing, and additional HA features.

```yaml
# Replica cluster (eu-central) with MaxScale
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-eu-central
spec:
  # [...]
  maxScaleRef:
    name: maxscale-eu-central
  replication:
    enabled: true
    gtidDomainId: 1
    semiSyncEnabled: false
  multiCluster:
    enabled: true
    members:
      - name: mariadb-eu-south
        externalMariaDbRef:
          name: mariadb-eu-south
      - name: mariadb-eu-central
        externalMariaDbRef:
          name: mariadb-eu-central
  # [...]
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: mariadb-eu-central
spec:
  host: maxscale-eu-central.default.svc.cluster.local
  port: 3306
```

Key differences:
- The `ExternalMariaDB` host points to the MaxScale service instead of the Pod FQDN.
- `spec.replication.semiSyncEnabled` must be `false` when connecting through MaxScale, as semi-synchronous replication is not supported through the router.
- The MaxScale instance must be configured with unique monitor names across clusters.

See the [replication-maxscale example](https://github.com/mariadb-operator/mariadb-operator/tree/main/examples/manifests/multi-cluster/replication-maxscale) for complete manifests.

#### Galera

This scenario uses Galera as the intra-cluster HA mechanism. Each MariaDB cluster runs a Galera cluster, and the clusters are connected via replication (Galera provides multi-master replication within the cluster, while inter-cluster replication is standard async/semi-sync replication).

```yaml
# Primary cluster (eu-south) with Galera
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-eu-south
spec:
  # [...]
  replicas: 3
  galera:
    enabled: true
    gtidDomainId: 0
    serverId: 1
  multiCluster:
    enabled: true
    primary: mariadb-eu-south
    members:
      - name: mariadb-eu-south
        externalMariaDbRef:
          name: mariadb-eu-south
      - name: mariadb-eu-central
        externalMariaDbRef:
          name: mariadb-eu-central
  # [...]
```

**Galera-specific considerations:**

- **GTID domain ID**: The Galera cluster uses `spec.galera.gtidDomainId` instead of `spec.replication.gtidDomainId`. The primary cluster uses `0`, and replica clusters use different values (e.g., `10`, `20`, etc.) to prevent GTID conflicts. Refer to [MariaDB docs](https://mariadb.com/docs/galera-cluster/high-availability/using-mariadb-replication-with-mariadb-galera-cluster/configuring-mariadb-replication-between-two-mariadb-galera-clusters) for additional documentation.
- **Server ID**: Each Galera node must have a unique `spec.galera.serverId`. The operator does not automatically increment server IDs for Galera clusters, so you must set them manually. Refer to [MariaDB docs](https://mariadb.com/docs/galera-cluster/high-availability/using-mariadb-replication-with-mariadb-galera-cluster/configuring-mariadb-replication-between-two-mariadb-galera-clusters) for additional documentation.
- **Replication configuration**: Galera uses `spec.galera.replPasswordSecretKeyRef` to configure the replication user, not `spec.replication.replica.replPasswordSecretKeyRef`.
- **Semi-synchronous replication**: Semi-synchronous replication is not supported with Galera. The operator automatically disables it when Galera is enabled.
- **Replication topology**: Galera provides synchronous multi-master replication within each cluster, while inter-cluster replication is asynchronous. This means the primary cluster's Galera nodes are fully synchronized with each other, and the replica cluster's primary replica replicates asynchronously from the primary cluster.

See the [galera example](https://github.com/mariadb-operator/mariadb-operator/tree/main/examples/manifests/multi-cluster/galera) for complete manifests.

#### Galera with MaxScale

Similar to the replication with MaxScale scenario, but using Galera as the intra-cluster HA mechanism. The replica cluster's primary replica connects to MaxScale instead of directly to the primary cluster's Pods.

See the [galera-maxscale example](https://github.com/mariadb-operator/mariadb-operator/tree/main/examples/manifests/multi-cluster/galera-maxscale) for complete manifests.

## Cluster switchover

Cluster switchover is the process of promoting a replica MariaDB cluster to become the new primary. This is useful for disaster recovery, migrations, or blue-green deployments. In multi-region HA setups, cluster switchover is used for failover. In blue-green deployments, cluster switchover is used to switch traffic from the old cluster to the newly upgraded cluster.

The switchover process consists of the following steps:

### Step 1: Enable maintenance mode on the primary cluster

Before initiating a cluster switchover, put the primary cluster in [maintenance mode](./maintenance.md). This prevents new writes from being accepted, allowing the replica cluster to fully sync.

> [!NOTE]
> When using MaxScale, enable maintenance mode on the `MaxScale` CR instead of the `MariaDB` CR. See the [MaxScale maintenance mode](./maintenance.md#maxscale-maintenance-mode) section for more information.

The recommended maintenance mode configuration is:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-eu-south
spec:
  # [...]
  maintenance:
    enabled: true
    cordon: true
    drainConnections: true
    drainGracePeriodSeconds: 30
    readOnly: true
  # [...]
```

This configuration:
- **Cordons** the cluster to block new connections.
- **Drains** long-running connections after the grace period.
- **Sets the database to read-only** to prevent any writes.

For more details on maintenance mode, see the [maintenance documentation](./maintenance.md).

Verify that the primary cluster is cordoned:

```bash
kubectl get mariadb mariadb-eu-south -o jsonpath="{.status}" | jq '{conditions: .conditions}'
{
  "conditions": [
    {
      "lastTransitionTime": "2026-05-25T15:43:37Z",
      "message": "Cordoned",
      "reason": "Cordoned",
      "status": "False",
      "type": "Ready"
    },
    # [...]
  ]
}
```

The `Ready` condition shows `status: "False"` with `reason: "Cordoned"`, indicating the cluster is in maintenance mode.

### Step 2: Wait for the replica cluster to sync

Verify that the replica cluster has fully synced with the primary cluster before proceeding. Check the replication status:

```bash
kubectl get mariadb mariadb-eu-central -o jsonpath="{.status.replication}" | jq
{
  "replicas": {
    "mariadb-eu-central-0": {
      "gtidCurrentPos": "0-10-4341,1-20-19",
      "gtidIOPos": "1-20-19,0-10-4341",
      "lastErrorTransitionTime": "2026-05-25T15:43:22Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true,
      "usingGtid": "Slave_Pos"
    },
    "mariadb-eu-central-1": {
      "gtidCurrentPos": "1-20-19",
      "gtidIOPos": "1-20-19",
      "lastErrorTransitionTime": "2026-05-24T07:47:56Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true,
      "usingGtid": "Slave_Pos"
    }
  },
  "roles": {
    "mariadb-eu-central-0": "PrimaryReplica",
    "mariadb-eu-central-1": "Replica"
  }
}
```

Ensure that `slaveIORunning` and `slaveSQLRunning` are `true` and `secondsBehindMaster` is `0` for the primary replica.

### Step 3: Trigger the cluster switchover

Update the `spec.multiCluster.primary` field on the replica cluster to its own name:

```bash
kubectl patch mariadb mariadb-eu-central --type merge -p '{"spec":{"multiCluster":{"primary":"mariadb-eu-central"}}}'
mariadb.k8s.mariadb.com/mariadb-eu-central patched
```

This tells the operator that the replica cluster should become the new primary. The operator will:

1. Reset the primary replica connection on all Pods of the old primary cluster.
2. Reconfigure GTIDs on the old primary cluster to only include its own domain ID.
3. Set the `status.currentMultiClusterPrimary` field to the new primary cluster.

Verify the switchover by checking both clusters:

```bash
kubectl get mariadb -o jsonpath='{range .items[*]}{.metadata.name}: primary={.status.currentPrimary}, multiClusterPrimary={.status.currentMultiClusterPrimary}{"\n"}{end}'
mariadb-eu-central: primary=mariadb-eu-central-0, multiClusterPrimary=mariadb-eu-central
mariadb-eu-south: primary=mariadb-eu-south-0, multiClusterPrimary=mariadb-eu-south
```

The new primary (`mariadb-eu-central`) has been promoted. The old primary (`mariadb-eu-south`) still shows itself as the multi-cluster primary because its `spec.multiCluster.primary` field still points to itself. Update it to point to the new primary:

```bash
kubectl patch mariadb mariadb-eu-south --type merge -p '{"spec":{"multiCluster":{"primary":"mariadb-eu-central"}}}'
mariadb.k8s.mariadb.com/mariadb-eu-south patched
```

After updating, verify that both clusters report the new primary:

```bash
kubectl get mariadb -o jsonpath='{range .items[*]}{.metadata.name}: primary={.status.currentPrimary}, multiClusterPrimary={.status.currentMultiClusterPrimary}{"\n"}{end}'
mariadb-eu-central: primary=mariadb-eu-central-0, multiClusterPrimary=mariadb-eu-central
mariadb-eu-south: primary=mariadb-eu-south-0, multiClusterPrimary=mariadb-eu-central
```

Check the replication roles to confirm the topology:

```bash
kubectl get mariadb mariadb-eu-central -o jsonpath="{.status.replication.roles}" | jq
{
  "mariadb-eu-central-0": "Primary",
  "mariadb-eu-central-1": "Replica"
}
```

```bash
kubectl get mariadb mariadb-eu-south -o jsonpath="{.status.replication.roles}" | jq
{
  "mariadb-eu-south-0": "PrimaryReplica",
  "mariadb-eu-south-1": "Replica"
}
```

The old primary (`mariadb-eu-south`) now has a `PrimaryReplica` Pod (`mariadb-eu-south-0`) that replicates from the new primary. Check its replication status:

```bash
kubectl get mariadb mariadb-eu-south -o jsonpath="{.status.replication}" | jq
{
  "replicas": {
    "mariadb-eu-south-0": {
      "gtidCurrentPos": "0-10-4343,1-20-19",
      "gtidIOPos": "1-20-19,0-10-4343",
      "lastErrorTransitionTime": "2026-05-25T15:44:12Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true,
      "usingGtid": "Slave_Pos"
    },
    "mariadb-eu-south-1": {
      "gtidCurrentPos": "0-10-4343",
      "gtidIOPos": "0-10-4343",
      "lastErrorTransitionTime": "2026-05-24T07:32:26Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true,
      "usingGtid": "Current_Pos"
    }
  },
  "roles": {
    "mariadb-eu-south-0": "PrimaryReplica",
    "mariadb-eu-south-1": "Replica"
  }
}
```

The operator continuously reconciles the multi-cluster topology and will automatically adjust GTIDs when the primary changes. During this process, GTIDs are filtered by domain ID to ensure that each cluster only replicates its own transactions, preventing GTID conflicts when the replica cluster becomes the new primary.

### Step 4: Update the LoadBalancer

After the switchover completes, update the externally managed LoadBalancer to point to the new primary cluster. This step must be performed manually.

The LoadBalancer should route traffic to either the MariaDB primary service or the MaxScale service, depending on your deployment configuration:

- **Without MaxScale**: Route to the MariaDB primary service (defined by `spec.primaryService`), which exposes the primary Pod directly. See the [high availability documentation](./high-availability.md) for more details on the primary service configuration.
- **With MaxScale**: Route to the MaxScale service, which provides connection pooling, query routing, and additional HA features. See the [MaxScale documentation](./maxscale.md) for more details on the MaxScale service configuration.

### Step 5: Disable maintenance mode on the old primary

Once the switchover is complete and traffic has been redirected, disable maintenance mode on the old primary cluster (now the replica) to bring it back into the topology as a replica of the new primary.

> [!NOTE]
> When using MaxScale, disable maintenance mode on the `MaxScale` CR instead of the `MariaDB` CR. See the [MaxScale maintenance mode](./maintenance.md#maxscale-maintenance-mode) section for more information.

```bash
kubectl patch mariadb mariadb-eu-south --type merge -p '{"spec":{"maintenance":{"enabled":false}}}'
mariadb.k8s.mariadb.com/mariadb-eu-south patched
```

Verify that the old primary is back to Running state:

```bash
kubectl get mariadb mariadb-eu-south -o jsonpath="{.status}" | jq '{conditions: .conditions, currentPrimary: .currentPrimary, currentMultiClusterPrimary: .currentMultiClusterPrimary}'
{
  "conditions": [
    {
      "lastTransitionTime": "2026-05-25T15:44:41Z",
      "message": "Running",
      "reason": "StatefulSetReady",
      "status": "True",
      "type": "Ready"
    },
    # [...]
  ],
  "currentPrimary": "mariadb-eu-south-0",
  "currentMultiClusterPrimary": "mariadb-eu-central"
}
```

The `Ready` condition shows `status: "True"`, indicating the cluster is back to normal operation. The `currentMultiClusterPrimary` confirms the cluster is now a replica of `mariadb-eu-central`.

## Limitations

The multi-cluster feature has the following limitations:

### External LoadBalancer

The external LoadBalancer that routes traffic to the primary cluster is not managed by the operator. It must be manually created and configured by the user, and must be manually updated to point to the new primary cluster after a cluster switchover. This applies to both multi-region HA and single-cluster blue-green deployments.

### Backups on primary only

Physical backups can only be taken from the primary cluster. This is because backups capture the GTID set of the cluster, and currently only backups with a single GTID domain ID are supported. Replica clusters have multiple GTID domain IDs (their own plus the domains they replicate from), which makes them incompatible with the current backup format.

As a consequence, replica clusters cannot be backed up directly. To recover a replica cluster from a failure, it must be re-bootstrapped from a backup taken in the primary cluster. This means the recovery point is determined by the last successful backup on the primary, not by the replica's current state. Plan your backup frequency accordingly to minimize potential data loss.

### MaxScale monitor names

When using MaxScale with multi-cluster, each MariaDB cluster must have a unique monitor name to avoid conflicts in the `mysql.maxscale_config` table. MaxScale stores monitor configurations in a shared database table, and duplicate monitor names would cause conflicts that prevent proper health checking and routing.

The operator generates monitor names automatically using the MaxScale CR name as a prefix (e.g., `<maxscale-name>-monitor`). To avoid conflicts, each MaxScale CR must have a unique name across all clusters. For example, use `maxscale-eu-south` for the primary cluster and `maxscale-eu-central` for the replica cluster. This applies to both multi-region and single-cluster deployments when MaxScale is used.

### Shared secrets

All MariaDB clusters in a multi-cluster topology must share the same secrets, specifically the root password and the TLS CA certificate. The root password secret is used by the replica cluster to connect to the primary cluster during bootstrapping and replication setup. The TLS CA certificate is used to establish trusted TLS connections between clusters.

For multi-region HA deployments, the same secrets must be available in all Kubernetes clusters. Any mismatch in secret data will prevent replication connections from being established.

For TLS, the CA certificate used to sign server certificates must be the same across all clusters. When configuring `spec.tls.serverCASecretRef` and `spec.tls.clientCASecretRef`, point to the same CA secret in each cluster. The operator uses this CA to validate TLS connections between clusters, and a mismatched CA will result in TLS handshake failures.

## Troubleshooting

### Checking multi-cluster status

The first step in troubleshooting a multi-cluster setup is to check the status of both the primary and replica MariaDB clusters:

```bash
kubectl get mariadb
NAME             READY   STATUS    PRIMARY                UPDATES                    AGE
mariadb-eu-south True    Running   mariadb-eu-south-0     ReplicasFirstPrimaryLast   31h
mariadb-eu-central True   Running   mariadb-eu-central-0   ReplicasFirstPrimaryLast   30h
```

Check the following fields:

- `status.currentPrimary`: The current primary Pod name within the cluster. For the primary cluster, this is `mariadb-eu-south-0`. For the replica cluster, this is the primary replica Pod (`mariadb-eu-central-0`).
- `status.currentMultiClusterPrimary`: The current primary cluster member name. In a healthy setup, both clusters report the same name (`mariadb-eu-south`).

```bash
kubectl get mariadb mariadb-eu-south -o jsonpath="{.status.currentPrimary}"
mariadb-eu-south-0
```

```bash
kubectl get mariadb mariadb-eu-central -o jsonpath="{.status.currentPrimary}"
mariadb-eu-central-0
```

```bash
kubectl get mariadb mariadb-eu-south -o jsonpath="{.status.currentMultiClusterPrimary}"
mariadb-eu-south
```

```bash
kubectl get mariadb mariadb-eu-central -o jsonpath="{.status.currentMultiClusterPrimary}"
mariadb-eu-south
```

### Checking replication roles

Check the replication roles of each cluster to verify the topology:

```bash
kubectl get mariadb mariadb-eu-south -o jsonpath="{.status.replication.roles}" | jq
{
  "mariadb-eu-south-0": "Primary",
  "mariadb-eu-south-1": "Replica"
}
```

```bash
kubectl get mariadb mariadb-eu-central -o jsonpath="{.status.replication.roles}" | jq
{
  "mariadb-eu-central-0": "PrimaryReplica",
  "mariadb-eu-central-1": "Replica"
}
```

The `PrimaryReplica` role is unique to the multi-cluster topology. It represents the Pod that acts as the replication source for the replica cluster's internal replicas, while itself replicating from the primary MariaDB cluster.

| Role | Description |
|------|-------------|
| `Primary` | The primary Pod in a MariaDB cluster. Handles all write operations. |
| `Replica` | A replica Pod in a MariaDB cluster. Replicates from the primary. |
| `PrimaryReplica` | The primary Pod in a replica MariaDB cluster. Replicates from the primary MariaDB cluster's primary. |
| `Unknown` | An unknown replication state. |

> [!NOTE]
> **Galera-specific behavior**: When using Galera as the intra-cluster HA mechanism, the replication status only shows the **primary replica** (the Pod that replicates from the primary cluster). Regular Galera nodes do not appear in the replication status because Galera handles its own clustering internally.

### Checking replication connections

To check the replication connections between clusters, inspect the replication status of the replica cluster:

```bash
kubectl get mariadb mariadb-eu-central -o jsonpath="{.status.replication}" | jq
{
  "replicas": {
    "mariadb-eu-central-0": {
      "gtidCurrentPos": "0-10-4337,1-20-11",
      "gtidIOPos": "0-10-4337",
      "lastErrorTransitionTime": "2026-05-24T07:47:56Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true,
      "usingGtid": "Slave_Pos"
    },
    "mariadb-eu-central-1": {
      "gtidCurrentPos": "0-10-4337,1-20-11",
      "gtidIOPos": "1-20-11,0-10-4337",
      "lastErrorTransitionTime": "2026-05-24T07:47:56Z",
      "lastIOErrno": 0,
      "lastIOError": "",
      "lastSQLErrno": 0,
      "lastSQLError": "",
      "secondsBehindMaster": 0,
      "slaveIORunning": true,
      "slaveSQLRunning": true,
      "usingGtid": "Slave_Pos"
    }
  },
  "roles": {
    "mariadb-eu-central-0": "PrimaryReplica",
    "mariadb-eu-central-1": "Replica"
  }
}
```

Key replication fields to check:

- `slaveIORunning` / `slaveSQLRunning`: Should be `true` for all replicas, including the primary replica. If `false`, the replication thread has stopped.
- `secondsBehindMaster`: The replication lag in seconds. A value of `0` indicates the replica is fully in sync.
- `lastIOError` / `lastSQLError`: Any recent errors from the I/O or SQL threads. Empty strings indicate no errors.
- `gtidCurrentPos`: The current GTID position. For the primary replica, this shows both domain `0` (replicated from the primary cluster) and domain `1` (its own cluster's transactions).
- `gtidIOPos`: The GTID position up to which the I/O thread has received events from the source. A difference between `gtidCurrentPos` and `gtidIOPos` indicates the I/O thread is behind.

### Checking ExternalMariaDB connectivity

To verify that the `ExternalMariaDB` resources are correctly configured and reachable:

```bash
kubectl get externalmariadb mariadb-eu-central -o jsonpath="{.status}" | jq
{
  "conditions": [
    {
      "lastTransitionTime": "2026-05-24T17:21:49Z",
      "message": "Healthy",
      "reason": "Healthy",
      "status": "True",
      "type": "Ready"
    }
  ]
}
```

Verify that the `status.conditions` shows `Ready: True` and `Healthy`.

### Rebuilding a replica cluster in bad state

When a replica cluster enters a bad state (e.g., data corruption, replication failure that cannot be recovered), the recommended approach is to rebuild it from a physical backup of the primary cluster. This process mirrors the provisioning steps described in [Provisioning process](#provisioning-process), with the key differences that you must delete the PVCs and MariaDB CR first, and point to an existing backup rather than creating a new one.

1. **Detect the issue**: Check the replication status to identify the problem. Look for `slaveIORunning` or `slaveSQLRunning` set to `false`, non-empty `lastIOError`/`lastSQLError`, error codes (`lastIOErrno`/`lastSQLErrno`) greater than `0`, continuously increasing `secondsBehindMaster`, or a `Ready` condition showing `False`. If the errors cannot be resolved by restarting replication threads or fixing the underlying issue (network, credentials, etc.), a rebuild is necessary.

2. **Delete the MariaDB CR**: Delete the replica cluster's `MariaDB` custom resource to stop the operator from managing the failing cluster:

   ```bash
   kubectl delete mariadb mariadb-eu-central
   ```

3. **Delete the PVCs**: Delete all PersistentVolumeClaims belonging to the replica cluster to remove the corrupted or broken data:

   ```bash
   kubectl delete pvc -l app.kubernetes.io/component=mariadb,app.kubernetes.io/instance=mariadb-eu-central
   ```

4. **Recreate the MariaDB CR**: Create a new `MariaDB` CR for the replica cluster, using the same configuration as the original replica (see [Step 3: Deploy replica cluster](#step-3-deploy-replica-cluster)) and pointing `spec.bootstrapFrom` to an existing physical backup in the primary cluster's S3 bucket. The operator will automatically download the backup, restore it to the new Pods, configure the internal replication topology, and establish the multi-cluster replication connection.

5. **Verify the rebuild**: Check that the `BackupRestored` and `ReplicationConfigured` conditions are present and `True`, and verify that `slaveIORunning` and `slaveSQLRunning` are `true`, `secondsBehindMaster` is `0`, and both `lastIOError` and `lastSQLError` are empty:

   ```bash
   kubectl get mariadb mariadb-eu-central -o jsonpath="{.status}" | jq '{conditions: .conditions, currentPrimary: .currentPrimary, currentMultiClusterPrimary: .currentMultiClusterPrimary}'
   kubectl get mariadb mariadb-eu-central -o jsonpath="{.status.replication}" | jq
   ```


