# High availability

Our recommended HA setup for production is:
- **[Galera](./GALERA.md)** with at least 3 replicas.
- Load balance requests using **[MaxScale](./MAXSCALE.md)** as database proxy.
- Configure **[pod anti affinity](#pod-anti-affinity)** to schedule your `Pods` in different Kubernetes `Nodes`.
- Define **[pod disruption budgets](#pod-disruption-budgets)**.
- Use **[dedicated nodes](#dedicated-nodes)** to avoid noisy neighbours.

Refer to the following sections for further detail.

## Supported HA modes

- **Single master HA via [SemiSync Replication](../examples/manifests/mariadb_replication.yaml)**: The primary node allows both reads and writes, while secondary nodes only allow reads.
- **Multi master HA via [Galera](./GALERA.md)**: All nodes support reads and writes. We have a designated primary where the writes are performed in order to avoid deadlocks.

## Kubernetes Services

In order to address nodes, `mariadb-operator` provides you with the following Kubernetes `Services`:
- `<mariadb-name>`: To be used for read requests. It will point to all nodes. 
- `<mariadb-name>-primary`: To be used for write requests. It will point to a single node, the primary.
- `<mariadb-name>-secondary`: To be used for read requests. It will point to all nodes, except the primary.

Whenever the primary changes, either by the user or by the operator, both the `<mariadb-name>-primary` and `<mariadb-name>-secondary` `Services` will be automatically updated by the operator to address the right nodes.

The primary may be manually changed by the user at any point by updating the `spec.[replication|galera].primary.podIndex` field. Alternatively,  automatic primary failover can be enabled by setting `spec.[replication|galera].primary.automaticFailover`, which will make the operator to switch primary whenever the primary `Pod` goes down.

## MaxScale

While Kubernetes `Services` can be utilized to dynamically address primary and secondary instances, the most robust high availability configuration we recommend relies on [MaxScale](https://mariadb.com/docs/server/products/mariadb-maxscale/). Please refer to [MaxScale docs](./MAXSCALE.md) for further details.

## Pod Anti Affinity

> [!WARNING]  
> Bear in mind that, when enabling this, you need to have at least as many `Nodes` available as the replicas specified in your `MariaDB`. Otherwise your `Pods` will be unscheduled and the cluster won't bootstrap.

To achieve real high availability, we need to run each of our `MariaDB` `Pods` in different Kubernetes `Nodes`. This way, we are considerably reducing the blast radius of `Nodes` being unavailable.

We can enable this by setting:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  affinity:
    enableAntiAffinity: true
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


## Dedicated Nodes

If you want to avoid noisy neighbours running in the same Kubernetes `Nodes` as your `MariaDB`, you may consider using dedicated `Nodes`. For achieving this, you need to taint your `Nodes` and add their counterpart tolerations in your `MariaDB` `Pods`.

Tainting your `Nodes` is not covered by this operator, it is something you need to do by yourself beforehand. You may take a look at the [Kubernetes documentation ](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) to understand how to achieve this.

Once your nodes are tainted, you can add tolerations to your `MariaDB` by setting:
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
``` 

## Reference
- [API reference](./API_REFERENCE.md)
- [Example suite](../examples/)
