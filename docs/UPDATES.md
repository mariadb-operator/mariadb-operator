# Updates

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.29

By leveraging the automation provided by `mariadb-operator`, you can declaratively manage large fleets of databases using CRs. This also covers day two operations, such as upgrades, which can be risky when rolling out updates to thousands of instances simultaneously.

To mitigate this, and to give you full control on the upgrade process, you are able to choose between multiple update strategies described in the following sections.

## Table of contents
<!-- toc -->
- [Update strategies](#update-strategies)
- [Configuration](#configuration)
- [Trigger updates](#trigger-updates)
- [`ReplicasFirstPrimaryLast`](#replicasfirstprimarylast)
- [`RollingUpdate`](#rollingupdate)
- [`OnDelete`](#ondelete)
- [`Never`](#never)
- [Data-plane updates](#data-plane-updates)
<!-- /toc -->

## Update strategies

In order to provide you with flexibility for updating `MariaDB` reliably, this operator supports multiple update strategies:

- [`ReplicasFirstPrimaryLast`](#replicasfirstprimarylast): Roll out replica `Pods` one by one, wait for each of them to become ready, and then proceed with the primary `Pod`.
- [`RollingUpdate`](#rollingupdate): Utilize the rolling update strategy from Kubernetes. 
- [`OnDelete`](#ondelete): Updates are performed manually by deleting `Pods`.
- [`Never`](#never): Pause updates.

## Configuration

The update strategy can be configured in the `updateStrategy` field of the `MariaDB` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  updateStrategy:
    type: ReplicasFirstPrimaryLast
``` 

It defaults to `ReplicasFirstPrimaryLast` if not provided.

## Trigger updates

Updates are not limited to updating the `image` field in the `MariaDB` resource, an update will be triggered whenever any field of the `Pod` template is changed. This translates into making changes to `MariaDB` fields that map directly or indirectly to the `Pod` template, for instance, the CPU and memory resources:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  ...
- image: mariadb:10.11.7
+ image: mariadb:10.11.8
  resources:
    requests:
      cpu: 200m
      memory: 128Mi
    limits:
-     memory: 1Gi
+     memory: 2Gi
```

Once the update is triggered, the operator manages it differently based on the selected update strategy.

## `ReplicasFirstPrimaryLast`

This role-aware update strategy consists in rolling out the replica `Pods` one by one first, waiting for each of them become ready (i.e. readiness probe passed), and then proceed with the primary `Pod`. This is the default update strategy, as it can potentially meet various reliability requirements and minimize the risks associated with updates:

- Write operations won't be affected until all the replica `Pods` have been rolled out. If something goes wrong in the update, such as an update to an incompatible MariaDB version, this is detected early when the replicas are being rolled out and the update operation will be paused at that point.
- Read operations impact is minimized by only rolling one replica `Pod` at a time.
- Waiting for every `Pod` to be synced minimizes the impact in the clustering protocols and the network.

## `RollingUpdate`

This strategy leverages the rolling update strategy from the [`StatefulSet` resource](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#rolling-updates), which, unlike [`ReplicasFirstPrimaryLast`](#replicasfirstprimarylast), does not take into account the role of the `Pods`(primary or replica). Instead, it rolls out the `Pods` one by one, from the highest to the lowest `StatefulSet` index.

You are able to pass extra parameters to this strategy via the `rollingUpdate` object:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  ...
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
``` 

## `OnDelete`

This strategy aims to provide a method to update `MariaDB` resources manually by allowing the user to restart the `Pods` individually. This way, the user has full control over the update process and can decide which `Pods` are rolled out at any given time.

Whenever an [update is triggered](#trigger-updates), the `MariaDB` will be marked as pending to update:

```bash
kubectl get mariadbs
NAME             READY   STATUS           PRIMARY            UPDATES    AGE
mariadb-galera   True    Pending update   mariadb-galera-0   OnDelete   5m17s
```

From this point, you are able to delete the `Pods` to trigger the update, which will result the `MariaDB` marked as updating:

```bash
kubectl get mariadbs
NAME             READY   STATUS         PRIMARY            UPDATES    AGE
mariadb-galera   True    Updating       mariadb-galera-0   OnDelete   9m50s
``` 

Once all the `Pods` have been rolled out, the `MariaDB` resource will be back to a ready state:

```bash
NAME             READY   STATUS         PRIMARY            UPDATES    AGE
mariadb-galera   True    Running        mariadb-galera-0   OnDelete   12m
```

## `Never`

The operator will not perform updates on the `StatefulSet` whenever this update strategy is configured. This could be useful in multiple scenarios:
- __Progressive fleet upgrades__: If you're managing large fleets of databases, you likely prefer to roll out updates progressively rather than simultaneously across all instances.
- __Operator upgrades__: When upgrading `mariadb-operator`, changes to the `StatefulSet` or the `Pod` template may occur from one version to another, which could trigger a rolling update of your `MariaDB` instances.

## Data-plane updates

Galera relies on [data-plane containers](./GALERA.md#data-plane) that run alongside MariaDB to implement provisioning and high availability operations on the cluster. These containers use the `mariadb-operator` image, which can be automatically updated by the operator based on its image version:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
    autoUpdateDataPlane: true
```

By default, `updateStrategy.autoUpdateDataPlane` is `false`, which means that no automatic upgrades will be performed, but you can opt-in/opt-out from this feature at any point in time by updating this field. For instance, you may want to selectively enable `updateStrategy.autoUpdateDataPlane` in a subset of your `MariaDB` instances after the operator has been upgraded to a newer version, and then disable it once the upgrades are completed.

It is important to note that this feature is fully compatible with the [`Never`](#never) strategy: no upgrades will happen when `updateStrategy.autoUpdateDataPlane=true` and `updateStrategy.type=Never`.