# High availability

This section provides guidance on how to configure high availability in `MariaDB` and `MaxScale` instances. If you are looking for an HA setup for the operator, please refer to the [Helm documentation](./helm.md#high-availability).

Our recommended setup for production is:
- Use a **[highly available topology](#highly-available-topologies)** for MariaDB:
  - Semi-synchronous **[replication](./replication.md)** with a primary node and at least 2 replicas.
  - Synchronous multi-master **[Galera](./galera.md)** with at least 3 nodes. Always an odd number of nodes, as it is quorum-based.
- Leverage **[MaxScale](./maxscale.md)** as database proxy to load balance requests and perform  failover/switchover operations. Configure 2 replicas to enable MaxScale upgrades without downtime.
- Use **[dedicated nodes](#dedicated-nodes)** to avoid noisy neighbours.
- Define **[pod disruption budgets](#pod-disruption-budgets)**.

Refer to the following sections for further detail.

## Table of contents
<!-- toc -->
- [Highly Available Topologies](#highly-available-topologies)
- [Kubernetes Services](#kubernetes-services)
- [MaxScale](#maxscale)
- [Pod Anti-Affinity](#pod-anti-affinity)
- [Dedicated Nodes](#dedicated-nodes)
- [Pod Disruption Budgets](#pod-disruption-budgets)
<!-- /toc -->

## Highly Available Topologies

- **[Semi-synchronous replication](./replication.md)**: The primary node allows both reads and writes, while secondary nodes only serve reads. Before committing the transaction back to the client, at least one replica should have sent an ACK to the primary node.
- **[Synchronous multi-master Galera](./galera.md)**: All nodes support reads and writes, but writes are only sent to one node to avoid contention. The fact that is synchronous and that all nodes are equally configured makes the primary failover/switchover operation seamless and usually instantaneous.

## Kubernetes Services

In order to address nodes, `mariadb-operator` provides you with the following Kubernetes `Services`:
- `<mariadb-name>`: This is the default `Service`, only intended for the [standalone topology](./standalone_mariadb.md).
- `<mariadb-name>-primary`: To be used for write requests. It will point to the primary node.
- `<mariadb-name>-secondary`: To be used for read requests. It will load balance requests to all nodes except the primary.

Whenever the primary changes, either by the user or by the operator, both the `<mariadb-name>-primary` and `<mariadb-name>-secondary` `Services` will be automatically updated by the operator to address the right nodes.

The primary may be manually changed by the user at any point by updating the `spec.[replication|galera].primary.podIndex` field. Alternatively,  automatic primary failover can be enabled by setting `spec.[replication|galera].primary.automaticFailover`, which will make the operator to switch primary whenever the primary `Pod` goes down.

## MaxScale

While Kubernetes `Services` can be used for addressing primary and secondary instances, we recommend utilizing [MaxScale](https://mariadb.com/docs/server/products/mariadb-maxscale/) as database proxy for doing so, as it comes with additional advantages:
- Enhanced failover/switchover operations for both replication and Galera
- Single entrypoint for both reads and writes
- Multiple router modules available to define how to route requests
- Replay pending transaction when primary goes down
- Ability to choose whether the old primary rejoins as a replica
- Connection pooling 

The full lifecyle of the MaxScale proxy is covered by this operator.  Please refer to [MaxScale docs](./maxscale.md) for further detail.

## Pod Anti-Affinity

> [!WARNING]  
> Bear in mind that, when enabling this, you need to have at least as many `Nodes` available as the replicas specified. Otherwise your `Pods` will be unscheduled and the cluster won't bootstrap.

To achieve real high availability, we need to run each `MariaDB` `Pod` in different Kubernetes `Nodes`. This practice, known as anti-affinity, helps reducing the blast radius of `Nodes` being unavailable.

By default, anti-affinity is disabled, which means that multiple `Pods` may be scheduled in the same `Node`, something not desired in HA scenarios.

You can selectively enable anti-affinity in all the different `Pods` managed by the `MariaDB` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  bootstrapFrom:
    restoreJob:
      affinity:
        antiAffinityEnabled: true
  ...
  metrics:
    exporter:
      affinity:
        antiAffinityEnabled: true
  ...
  affinity:
    antiAffinityEnabled: true
```

Anti-affinity may also be enabled in the the resources that have a reference to `MariaDB`, resulting in their `Pods` being scheduled in `Nodes` where `MariaDB` is not running. For instance, the `Backup` and `Restore` processes can run in different `Nodes`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup
spec:
  mariaDbRef:
    name: mariadb-galera
  ...
  affinity:
    antiAffinityEnabled: true
```

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Restore
metadata:
  name: restore
spec:
  mariaDbRef:
    name: mariadb-galera
  ...
  affinity:
    antiAffinityEnabled: true
```

In the case of `MaxScale`, the `Pods` will also be placed in `Nodes` isolated in terms of compute, ensuring isolation not only among themselves but also from the `MariaDB` `Pods`. For example, if you run a `MariaDB` and `MaxScale` with 3 replicas each, you will need 6 `Nodes` in total:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  mariaDbRef:
    name: mariadb-galera
  ...
  metrics:
    exporter:
      affinity:
        antiAffinityEnabled: true
  ...
  affinity:
    antiAffinityEnabled: true
```

Default anti-affinity rules generated by the operator might not satisfy your needs, but you can always define your own rules. For example, if you want the `MaxScale` `Pods` to be in different `Nodes`, but you want them to share `Nodes` with `MariaDB`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  mariaDbRef:
    name: mariadb-galera
  ...
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/instance
            operator: In
            values:
            - maxscale-galera
            # 'mariadb-galera' instance omitted (default anti-affinity rule)
        topologyKey: kubernetes.io/hostname
```

## Dedicated Nodes

If you want to avoid noisy neighbours running in the same Kubernetes `Nodes` as your `MariaDB`, you may consider using dedicated `Nodes`. For achieving this, you will need:
- Taint your `Nodes` and add the counterpart toleration in your `Pods`.
> [!IMPORTANT]  
> Tainting your `Nodes` is not covered by this operator, it is something you need to do by yourself beforehand. You may take a look at the [Kubernetes documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) to understand how to achieve this.
- Select the `Nodes` to schedule in via a `nodeSelector` in your `Pods`.
> [!NOTE]  
> Although you can use the default `Node` labels, you may consider adding more significative labels to your `Nodes`, as you will have to refer to them in your `Pod` `nodeSelector`. Refer to the [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/#add-a-label-to-a-node). 

- Add `podAntiAffinity` to your `Pods` as described in the [Pod Anti-Affinity](#pod-anti-affinity) section.


Once you have completed the previous steps, you can configure your `MariaDB` as follows:
```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  tolerations:
    - key: "k8s.mariadb.com/ha"
      operator: "Exists"
      effect: "NoSchedule"
  nodeSelector:
    "k8s.mariadb.com/node": "ha" 
  affinity:
    antiAffinityEnabled: true
```

## Pod Disruption Budgets

> [!IMPORTANT]  
> Take a look at the [Kubernetes documentation](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) if you are unfamiliar to `PodDisruptionBudgets`

By defining a `PodDisruptionBudget`, you are telling Kubernetes how many `Pods` your database tolerates to be down. This quite important for planned maintenance operations such as `Node` upgrades.

`mariadb-operator` creates a default `PodDisruptionBudget` if you are running in HA, but you are able to define your own by setting:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
    podDisruptionBudget:
      maxUnavailable: 33%
```
