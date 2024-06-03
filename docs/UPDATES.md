# Updates

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.26

This documentation aims to describe the supported strategies to perform updates of the `MariaDB` resource. 

## Table of contents
<!-- toc -->
- [Modes](#modes)
- [Configuration](#configuration)
- [Trigger updates](#trigger-updates)
- [<code>ReplicasFirstPrimaryLast</code>](#replicasfirstprimarylast)
- [<code>RollingUpdateUpdateType</code>](#rollingupdateupdatetype)
- [<code>OnDeleteUpdateType</code>](#ondeleteupdatetype)
<!-- /toc -->

## Update strategies

In order to provide you with flexibility for updating `MariaDB` reliably, this operator supports multiple update strategies:

- [`ReplicasFirstPrimaryLast`](#replicasfirstprimarylast): Roll out replica `Pods` one by one first, and then update the primary `Pod`. This is the recommended mode.
- [`RollingUpdate`](#rollingupdate): Utilize the default Kubernetes rolling update strategy. 
- [`OnDelete`](#ondelete): Updates are performed manually by deleting `Pods`.

## Configuration

The update strategy can be configured in the `updateStrategy` field of the `MariaDB` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  ...
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
  resources:
    requests:
      cpu: 200m
      memory: 128Mi
    limits:
-     memory: 1Gi
+     memory: 2Gi
```

Once the update is triggered, they way it is managed by the operator varies depending of the chosen update strategy.

## `ReplicasFirstPrimaryLast`

This strategy consists in rolling out the replica `Pods` one by one first, waiting until they are fully in sync and then proceed with the primary `Pod` which handles the write operations. This is the default update strategy, as it could potentially fit satisfy multiple reliability requirements and minimize the risk of updates:
- Write operations won't be affected until all the replica `Pods` have been rolled out. If something goes wrong in the update, for example,  update to an incompatible MariaDB version, this is detected early when the replicas are being rolled out and the update operation will be paused from that point
- Read operations impact is minimized by only rolling just one replica `Pod` at a time.
- Waiting for every `Pod` to be synced (readiness probe passed) minimizes the impact in the clustering protocols and the network.


## `RollingUpdate`

## `OnDelete`