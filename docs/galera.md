# High availability via Galera

The `mariadb-operator` provides cloud native support for provisioning and operating multi-master MariaDB clusters using Galera. This setup enables the ability to perform both read and write operations on all nodes, enhancing availability and allowing scalability across multiple nodes.

In certain circumstances, it could be the case that all the nodes of your cluster go down at the same time, something that Galera is not able to recover by itself, and it requires manual action to bring the cluster up again, as documented in the [Galera documentation](https://galeracluster.com/library/documentation/crash-recovery.html). Luckly enough, `mariadb-operator` has you covered and it encapsulates this operational expertise in the `MariaDB` CRD. You just need to declaratively specify `spec.galera`, as explained in more detail [later in this guide](#configuration).

To accomplish this, after the MariaDB cluster has been provisioned, `mariadb-operator` will regularly monitor the cluster's status to make sure it is healthy. If any issues are detected, the operator will initiate the [recovery process](#galera-cluster-recovery) to restore the cluster to a healthy state. During this process, the operator will set status conditions in the `MariaDB` and emit `Events` so you have a better understanding of the recovery progress and the underlying activities being performed. For example, you may want to know which `Pods` were out of sync to further investigate infrastructure-related issues (i.e. networking, storage...) on the nodes where these `Pods` were scheduled.

## Table of contents
<!-- toc -->
- [Data-plane](#data-plane)
- [<code>MariaDB</code> configuration](#mariadb-configuration)
- [Storage](#storage)
- [Wsrep provider](#wsrep-provider)
- [IPv6 support](#ipv6-support)
- [Agent auth methods](#agent-auth-methods)
- [Backup and restore](#backup-and-restore)
- [Galera cluster recovery](#galera-cluster-recovery)
- [Bootstrap Galera cluster from existing PVCs](#bootstrap-galera-cluster-from-existing-pvcs)
- [Quickstart](#quickstart)
- [Troubleshooting](#troubleshooting)
- [Reference](#reference)
<!-- /toc -->

## Data-plane

To be able to effectively provision and recover MariaDB Galera clusters, the following data-plane components were introduced to run alongside MariaDB and co-operate with `mariadb-operator`:
- **init**: Init container that dynamically provisions the Galera configuration file before the MariaDB container starts. Guarantees ordered deployment of `Pods` even if `spec.podManagementPolicy=Parallel` is set on the MariaDB `StatefulSet`, something crucial for performing the Galera recovery, as the operator needs to restart `Pods` independently.
- **agent**: Sidecar agent that exposes the Galera state ([`grastate.dat`](https://galeracluster.com/2016/11/introducing-the-safe-to-bootstrap-feature-in-galera-cluster/)) via HTTP and allows the operator to remotely bootstrap and recover the Galera cluster. It comes with [multiple auth methods](#agent-auth-methods) to ensure that only the operator is able to call the agent.

All these components are available in the operator image. More preciselly, they are subcommands of the CLI shipped as binary inside the image.

## `MariaDB` configuration

The easiest way to get a MariaDB Galera cluster up and running is setting `spec.galera.enabled = true`, like in this [example](../examples/manifests/mariadb_galera.yaml):

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
...
  replicas: 3

  galera:
    enabled: true
```

This relies on sensible defaults set by the operator, which may not be suitable for your Kubernetes cluster. This can be solved by overriding the defaults, as in this other [example](../examples/manifests/mariadb_galera.yaml), so you have fine-grained control over the Galera configuration.

Refer to the [reference](#reference) section to better understand the purpose of each field.

## Storage

By default, `mariadb-operator` provisions two PVCs for running Galera:
- **Storage PVC**: Used to back the MariaDB data directory, mounted at `/var/lib/mysql`.
- **Config PVC**: Where the Galera config files are located, mounted at `/etc/mysql/conf.d`.

However, you are also able to use just one PVC for keeping both the data and the config files:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  galera:
    enabled: true
    config:
      reuseStorageVolume: true
```

## Wsrep provider

You are able to pass extra options to the Galera wsrep provider by using the `galera.providerOptions` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  galera:
    providerOptions:
      gcs.fc_limit: '64'
```

It is important to note that, the `ist.recv_addr` cannot be set by the user, as it is automatically configured to the `Pod` IP by the operator, something that an user won't be able to know beforehand.

A list of the available options can be found in the [MariaDB documentation](https://mariadb.com/kb/en/wsrep_provider_options/).

## IPv6 support

If you have a Kubernetes cluster running with IPv6, the operator will automatically detect the IPv6 addresses of your `Pods` and it will configure several [wsrep provider](#wsrep-provider) options to ensure that the Galera protocol runs smoothly with IPv6.

## Agent auth methods

As previously mentioned in the [data-plane](#data-plane) section, the agent exposes an API to remotely manage the MariaDB Galera cluster. The following authentication methods are supported to ensure that only the operator is able to call the agent:

#### `ServiceAccount` based authentication

The operator uses its `ServiceAccount` token as a mean of  authentication for communicating with the agent, which subsequently verifies the token by creating a [`TokenReview` object](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-review-v1/). This is the default authentication method and will be automatically applied by setting:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  galera:
    agent:
      kubernetesAuth:
        enabled: true
```
This Kubernetes-native authentication mechanism eliminates the need for the operator to manage credentials, as it relies entirely on Kubernetes for this purpose. However, the drawback is that the agent requires cluster-wide permissions to impersonate the [`system:auth-delegator`](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#other-component-roles) `ClusterRole` and to create [`TokenReviews`](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-review-v1/), which are cluster-scoped objects.

#### Basic authentication

As an alternative, the agent also supports basic authentication:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  galera:
    agent:
      basicAuth:
        enabled: true
```

Unlike the [`ServiceAccount` based authentication](#serviceaccount-based-authentication), the operator needs to explicitly generate credentials to authenticate. The advantage of this approach is that it is entirely decoupled from Kubernetes and it does not require cluster-wide permissions on the Kubernetes API.

## Backup and restore

Please refer to the [backup documentation](./logical_backup.md) to understand how to backup and restore Galera clusters. Specially, make sure you understand the [Galera backup limitations](./logical_backup.md#galera-backup-limitations).


## Galera cluster recovery

`mariadb-operator` is able to monitor the Galera cluster and act accordinly to recover it if needed. This feature is enabled by default, but you may tune it as you need:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  galera:
    enabled: true
    recovery:
      enabled: true
      minClusterSize: 1
      clusterMonitorInterval: 10s
      clusterHealthyTimeout: 30s
      clusterBootstrapTimeout: 10m
      podRecoveryTimeout: 5m
      podSyncTimeout: 5m
```

The `minClusterSize` field indicates the minimum cluster size (either absolut number of replicas or percentage) for the operator to consider the cluster healthy. If the cluster is unhealthy for more than the period defined in `clusterHealthyTimeout` (`30s` by default), a cluster recovery process is initiated by the operator. The process is explained in the [Galera documentation](https://galeracluster.com/library/documentation/crash-recovery.html) and consists of the following steps:

- Recover the sequence number from the `grastate.dat` on each node.
- Trigger a [recovery `Job`](#galera-recovery-job) to obtain the sequence numbers in case that the previous step didn't manage to.
- Mark the node with highest sequence (bootstrap node) as safe to bootstrap.
- Bootstrap a new cluster in the bootstrap node.
- Restart and wait until the bootstrap node becomes ready.
- Restart the rest of the nodes one by one so they can join the new cluster.

The operator monitors the Galera cluster health periodically and performs the cluster recovery described above if needed. You are able to tune the monitoring interval via the `clusterMonitorInterval` field.

Refer to the [reference](#reference) section to better understand the purpose of each field.

#### Galera recovery `Job`

During the recovery process, a `Job` is triggered for each `MariaDB` `Pod` to obtain the sequence numbers. It's crucial for this `Job` to succeed; otherwise, the recovery process will fail. As a user, you are responsible for adjusting this `Job` to allocate sufficient resources and provide the necessary metadata to ensure its successful completion.

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  galera:
    enabled: true
    recovery:
      job:
        metadata:
          labels:
            sidecar.istio.io/inject: "false"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            memory: 256Mi
```

For example, if you're using a service mesh like Istio, it's important to add the `sidecar.istio.io/inject=false` label. Without this label, the `Job` will not complete, which would prevent the recovery process from finishing successfully.

#### Force cluster bootstrap
 
> [!CAUTION]
> Use this option only in exceptional circumstances. Not selecting the `Pod` with the highest sequence number may result in data loss.

> [!WARNING]
> Ensure you unset `forceClusterBootstrapInPod` after completing the bootstrap to allow the operator to choose the appropriate `Pod` to bootstrap from in an event of cluster recovery.

You have the ability to manually select which `Pod` is used to bootstrap a new cluster during the recovery process by setting `forceClusterBootstrapInPod`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  ...
  galera:
    enabled: true
    recovery:
      enabled: true
      forceClusterBootstrapInPod: "mariadb-galera-0"
```

This should only be used in exceptional circumstances:
- You are absolutely certain that the chosen `Pod` has the highest sequence number.
- The operator has not yet selected a `Pod` to bootstrap from.

You can verify this with the following command:

```bash
kubectl get mariadb mariadb-galera -o jsonpath="{.status.galeraRecovery}" | jq
{
  "recovered": {
    "mariadb-galera-0": {
      "seqno": 350454,
      "uuid": "67a44ea9-63a8-11ef-98a2-2b0c0aa0a627"
    },
    "mariadb-galera-1": {
      "seqno": 350450,
      "uuid": "67a44ea9-63a8-11ef-98a2-2b0c0aa0a627"
    }
  },
  "state": {
    "mariadb-galera-0": {
      "safeToBootstrap": false,
      "seqno": -1,
      "uuid": "67a44ea9-63a8-11ef-98a2-2b0c0aa0a627",
      "version": "2.1"
    },
    "mariadb-galera-1": {
      "safeToBootstrap": false,
      "seqno": -1,
      "uuid": "67a44ea9-63a8-11ef-98a2-2b0c0aa0a627",
      "version": "2.1"
    },
    "mariadb-galera-2": {
      "safeToBootstrap": false,
      "seqno": -1,
      "uuid": "67a44ea9-63a8-11ef-98a2-2b0c0aa0a627",
      "version": "2.1"
    }
  }
}
```

In this case, assuming that `mariadb-galera-2` sequence is lower than `350454`, it should be safe to bootstrap from `mariadb-galera-0`.

Finally, after your cluster has been bootstrapped, remember to unset `forceClusterBootstrapInPod` to allow the operator to select the appropriate node for bootstrapping in the event of a cluster recovery.

## Bootstrap Galera cluster from existing PVCs

`mariadb-operator` will never delete your `MariaDB` PVCs. Whenever you delete a `MariaDB` resource, the PVCs will remain intact so you could reuse them to re-provision a new cluster.

That said, Galera is unable to form a cluster from pre-existing state, it requires a [cluster recovery](#galera-cluster-recovery) process to identify which `Pod` has the highest sequence number to bootstrap a new cluster. That's exactly what the operator does: whenever a new `MariaDB` Galera cluster is created and previously created PVCs exist, a cluster recovery process is automatically triggered.

## Quickstart

First of all, install the following configuration manifests that will be referenced by the CRDs further:
```bash
kubectl apply -f examples/manifests/config
```
Next, you can proceed with the installation of a `MariaDB` instance with Galera support:
```bash
kubectl apply -f examples/manifests/mariadb_galera.yaml
```
```bash
kubectl get mariadbs
NAME             READY   STATUS    PRIMARY POD          AGE
mariadb-galera   True    Running   mariadb-galera-0     48m

kubectl get events --field-selector involvedObject.name=mariadb-galera --sort-by='.lastTimestamp'
LAST SEEN   TYPE     REASON                 OBJECT                               MESSAGE
...
45m         Normal   GaleraClusterHealthy   mariadb/mariadb-galera               Galera cluster is healthy

kubectl get mariadb mariadb-galera -o jsonpath="{.status.conditions[?(@.type=='GaleraReady')]}"
{"lastTransitionTime":"2023-07-13T18:22:31Z","message":"Galera ready","reason":"GaleraReady","status":"True","type":"GaleraReady"}

kubectl get mariadb mariadb-galera -o jsonpath="{.status.conditions[?(@.type=='GaleraConfigured')]}"
{"lastTransitionTime":"2023-07-13T18:22:31Z","message":"Galera configured","reason":"GaleraConfigured","status":"True","type":"GaleraConfigured"}

kubectl get statefulsets -o wide
NAME             READY   AGE   CONTAINERS      IMAGES
mariadb-galera   3/3     58m   mariadb,agent   mariadb:11.0.3,ghcr.io/mariadb-operator/mariadb-operator:v0.0.26

kubectl get pods -o wide
NAME                                        READY   STATUS    RESTARTS   AGE   IP           NODE          NOMINATED NODE   READINESS GATES
mariadb-galera-0                            2/2     Running   0          58m   10.244.2.4   mdb-worker3   <none>           <none>
mariadb-galera-1                            2/2     Running   0          58m   10.244.1.9   mdb-worker2   <none>           <none>
mariadb-galera-2                            2/2     Running   0          58m   10.244.5.4   mdb-worker4   <none>           <none>
```
Up and running. Let's now proceed with simulating a Galera cluster failure by deleting all the `Pods` at the same time:
```bash
kubectl delete pods -l app.kubernetes.io/instance=mariadb-galera
pod "mariadb-galera-0" deleted
pod "mariadb-galera-1" deleted
pod "mariadb-galera-2" deleted
```
After some time, we will see the `MariaDB` entering a non `Ready` state:
```bash
kubectl get mariadb mariadb-galera
NAME             READY   STATUS             PRIMARY POD             AGE
mariadb-galera   False   Galera not ready   mariadb-galera-0        67m

kubectl get events --field-selector involvedObject.name=mariadb-galera --sort-by='.lastTimestamp'
LAST SEEN   TYPE      REASON                    OBJECT                       MESSAGE
...
48s         Warning   GaleraClusterNotHealthy   mariadb/mariadb-galera       Galera cluster is not healthy

kubectl get mariadb mariadb-galera -o jsonpath="{.status.conditions[?(@.type=='GaleraReady')]}"
{"lastTransitionTime":"2023-07-13T19:25:17Z","message":"Galera not ready","reason":"GaleraNotReady","status":"False","type":"GaleraReady"}
```
Eventually, the operator will kick in and recover the Galera cluster:
```bash
kubectl get events --field-selector involvedObject.name=mariadb-galera --sort-by='.lastTimestamp'
LAST SEEN   TYPE      REASON                    OBJECT                       MESSAGE
...
16m         Warning   GaleraClusterNotHealthy   mariadb/mariadb-galera       Galera cluster is not healthy
16m         Normal    GaleraPodStateFetched     mariadb/mariadb-galera       Galera state fetched in Pod 'mariadb-galera-2'
16m         Normal    GaleraPodStateFetched     mariadb/mariadb-galera       Galera state fetched in Pod 'mariadb-galera-1'
16m         Normal    GaleraPodStateFetched     mariadb/mariadb-galera       Galera state fetched in Pod 'mariadb-galera-0'
16m         Normal    GaleraPodRecovered        mariadb/mariadb-galera       Recovered Galera sequence in Pod 'mariadb-galera-1'
16m         Normal    GaleraPodRecovered        mariadb/mariadb-galera       Recovered Galera sequence in Pod 'mariadb-galera-2'
17m         Normal    GaleraPodRecovered        mariadb/mariadb-galera       Recovered Galera sequence in Pod 'mariadb-galera-0'
17m         Normal    GaleraClusterBootstrap    mariadb/mariadb-galera       Bootstrapping Galera cluster in Pod 'mariadb-galera-2'
20m         Normal    GaleraClusterHealthy      mariadb/mariadb-galera       Galera cluster is healthy

kubectl get mariadb mariadb-galera -o jsonpath="{.status.galeraRecovery}"
{"bootstrap":{"pod":"mariadb-galera-2","time":"2023-07-13T19:25:28Z"},"recovered":{"mariadb-galera-0":{"seqno":3,"uuid":"bf00b9c3-21a9-11ee-984f-9ba9ff0e9285"},"mariadb-galera-1":{"seqno":3,"uuid":"bf00b9c3-21a9-11ee-984f-9ba9ff0e9285"},"mariadb-galera-2":{"seqno":3,"uuid":"bf00b9c3-21a9-11ee-984f-9ba9ff0e9285"}},"state":{"mariadb-galera-0":{"safeToBootstrap":false,"seqno":-1,"uuid":"bf00b9c3-21a9-11ee-984f-9ba9ff0e9285","version":"2.1"},"mariadb-galera-1":{"safeToBootstrap":false,"seqno":-1,"uuid":"bf00b9c3-21a9-11ee-984f-9ba9ff0e9285","version":"2.1"},"mariadb-galera-2":{"safeToBootstrap":false,"seqno":-1,"uuid":"bf00b9c3-21a9-11ee-984f-9ba9ff0e9285","version":"2.1"}}}
```
Finally, the `MariaDB` resource will become `Ready` and your Galera cluster will be operational again:
```bash
kubectl get mariadb mariadb-galera -o jsonpath="{.status.conditions[?(@.type=='GaleraReady')]}"
{"lastTransitionTime":"2023-07-13T19:27:51Z","message":"Galera ready","reason":"GaleraReady","status":"True","type":"GaleraReady"}

kubectl get mariadb mariadb-galera
NAME             READY   STATUS    PRIMARY POD          AGE
mariadb-galera   True    Running   mariadb-galera-0     82m
```

## Troubleshooting

The aim of this section is showing you how to diagnose your Galera cluster when something goes wrong. In this situations, observability is a key factor to understand the problem, so we recommend following these steps before jumping into debugging the problem.

- Inspect `MariaDB` status conditions.
```bash
kubectl get mariadb mariadb-galera -o jsonpath="{.status}"
{"conditions":[{"lastTransitionTime":"2023-08-05T14:58:57Z","message":"Galera not ready","reason":"GaleraNotReady","status":"False","type":"Ready"},{"lastTransitionTime":"2023-08-05T14:58:57Z","message":"Galera not ready","reason":"GaleraNotReady","status":"False","type":"GaleraReady"},{"lastTransitionTime":"2023-08-03T19:21:16Z","message":"Galera configured","reason":"GaleraConfigured","status":"True","type":"GaleraConfigured"}],"currentPrimary":"All","galeraRecovery":{"bootstrap":{"pod":"mariadb-galera-1","time":"2023-08-05T14:59:18Z"},"recovered":{"mariadb-galera-0":{"seqno":17,"uuid":"6ea235ec-3232-11ee-8152-4af03d2c43a9"},"mariadb-galera-1":{"seqno":17,"uuid":"6ea235ec-3232-11ee-8152-4af03d2c43a9"},"mariadb-galera-2":{"seqno":16,"uuid":"6ea235ec-3232-11ee-8152-4af03d2c43a9"}},"state":{"mariadb-galera-0":{"safeToBootstrap":false,"seqno":-1,"uuid":"6ea235ec-3232-11ee-8152-4af03d2c43a9","version":"2.1"},"mariadb-galera-1":{"safeToBootstrap":false,"seqno":-1,"uuid":"6ea235ec-3232-11ee-8152-4af03d2c43a9","version":"2.1"},"mariadb-galera-2":{"safeToBootstrap":false,"seqno":-1,"uuid":"6ea235ec-3232-11ee-8152-4af03d2c43a9","version":"2.1"}}}}
```
- Make sure network connectivity is fine by checking that you have an `Endpoint` per `Pod` in your Galera cluster.
```bash
kubectl get endpoints mariadb-galera-internal -o yaml
apiVersion: v1
kind: Endpoints
metadata:
  name: mariadb-internal
subsets:
- addresses:
  - hostname: mariadb-1
    ip: 10.255.140.181
    nodeName: k8s-worker-1
    targetRef:
      kind: Pod
      name: mariadb-1
      namespace: mariadb
  - hostname: mariadb-2
    ip: 10.255.20.156
    nodeName: k8s-worker-2
    targetRef:
      kind: Pod
      name: mariadb-2
      namespace: mariadb
  - hostname: mariadb-0
    ip: 10.255.214.164
    nodeName: k8s-worker-0
    targetRef:
      kind: Pod
      name: mariadb-0
      namespace: mariadb
  ports:
  - name: sst
    port: 4568
    protocol: TCP
  - name: ist
    port: 4567
    protocol: TCP
  - name: mariadb
    port: 3306
    protocol: TCP
  - name: agent
    port: 5555
    protocol: TCP
  - name: cluster
    port: 4444
    protocol: TCP

```
- Check the events associated with the `MariaDB` object, as they provide significant insights for diagnosis, particularly within the context of cluster recovery.
```bash
kubectl get events --field-selector involvedObject.name=mariadb-galera --sort-by='.lastTimestamp'
LAST SEEN   TYPE      REASON                    OBJECT                       MESSAGE
...
16m         Warning   GaleraClusterNotHealthy   mariadb/mariadb-galera       Galera cluster is not healthy
16m         Normal    GaleraPodStateFetched     mariadb/mariadb-galera       Galera state fetched in Pod 'mariadb-galera-2'
16m         Normal    GaleraPodStateFetched     mariadb/mariadb-galera       Galera state fetched in Pod 'mariadb-galera-1'
16m         Normal    GaleraPodStateFetched     mariadb/mariadb-galera       Galera state fetched in Pod 'mariadb-galera-0'
16m         Normal    GaleraPodRecovered        mariadb/mariadb-galera       Recovered Galera sequence in Pod 'mariadb-galera-1'
16m         Normal    GaleraPodRecovered        mariadb/mariadb-galera       Recovered Galera sequence in Pod 'mariadb-galera-2'
17m         Normal    GaleraPodRecovered        mariadb/mariadb-galera       Recovered Galera sequence in Pod 'mariadb-galera-0'
17m         Normal    GaleraClusterBootstrap    mariadb/mariadb-galera       Bootstrapping Galera cluster in Pod 'mariadb-galera-2'
20m         Normal    GaleraClusterHealthy      mariadb/mariadb-galera       Galera cluster is healthy
```

- Enable `debug` logs in `mariadb-operator`.

```bash
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --set logLevel=debug
kubectl logs mariadb-operator-546c78f4f5-gq44k
{"level":"info","ts":1691090524.4911606,"logger":"galera.health","msg":"Checking Galera cluster health","controller":"statefulset","controllerGroup":"apps","controllerKind":"StatefulSet","statefulSet":{"name":"mariadb-galera","namespace":"default"},"namespace":"default","name":"mariadb-galera","reconcileID":"098620db-4486-45cc-966a-9f3fec0d165e"}
{"level":"debug","ts":1691090524.4911761,"logger":"galera.health","msg":"StatefulSet ready replicas","controller":"statefulset","controllerGroup":"apps","controllerKind":"StatefulSet","statefulSet":{"name":"mariadb-galera","namespace":"default"},"namespace":"default","name":"mariadb-galera","reconcileID":"098620db-4486-45cc-966a-9f3fec0d165e","replicas":1}
```

- Get the logs of all the `MariaDB` `Pod` containers, not only of the main `mariadb` container but also the `agent` and `init` ones.
  
```bash
kubectl logs mariadb-galera-0 -c init
{"level":"info","ts":1691090778.5239124,"msg":"Starting init"}
{"level":"info","ts":1691090778.5305626,"msg":"Configuring Galera"}
{"level":"info","ts":1691090778.5307593,"msg":"Already initialized. Init done"}

kubectl logs mariadb-galera-0 -c agent
{"level":"info","ts":1691090779.3193653,"logger":"server","msg":"server listening","addr":":5555"}
2023/08/03 19:26:28 "POST http://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local:5555/api/recovery HTTP/1.1" from 10.244.4.2:39162 - 200 58B in 4.112086ms
2023/08/03 19:26:28 "DELETE http://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local:5555/api/recovery HTTP/1.1" from 10.244.4.2:39162 - 200 0B in 883.544µs

kubectl logs mariadb-galera-0 -c mariadb
2023-08-03 19:27:10 0 [Note] WSREP: Member 2.0 (mariadb-galera-0) synced with group.
2023-08-03 19:27:10 0 [Note] WSREP: Processing event queue:...100.0% (1/1 events) complete.
2023-08-03 19:27:10 0 [Note] WSREP: Shifting JOINED -> SYNCED (TO: 6)
2023-08-03 19:27:10 2 [Note] WSREP: Server mariadb-galera-0 synced with group
2023-08-03 19:27:10 2 [Note] WSREP: Server status change joined -> synced
2023-08-03 19:27:10 2 [Note] WSREP: Synchronized with group, ready for connections
```

Once you are done with these steps, you will have the context required to jump ahead to the [Common errors](#common-errors) section to see if any of them matches your case.  If they don't, feel free to open an issue or even a PR updating this document if you managed to resolve it.

### Common errors

#### Galera cluster recovery not progressing

If your `MariaDB` Galera cluster has been in `GaleraNotReady` state for a long time, the recovery process might not be progressing. You can diagnose this by checking:
- Operator logs.
- Galera recovery status:
```bash
kubectl get mariadb mariadb-galera -o jsonpath="{.status.galeraRecovery}" | jq
```
- `MariaDB` events:
```bash
kubectl get events --field-selector involvedObject.name=mariadb-galera
```
- If you have `Pods` named `<mariadb-name>-<ordinal>-recovery-<suffix>` running for a long time, check its logs to understand if something is wrong.

One of the reasons could be misconfigured Galera recovery `Jobs`, please make sure you read [this section](#galera-recovery-job). If after checking all the points above, there are still no clear symptoms of what could be wrong, continue reading.

First af all, you could attempt to forcefully bootstrap a new cluster as it is described in [this section](#force-cluster-bootstrap). Please, refrain from doing so if the conditions described in the docs are not met.

Alternatively, if you can afford some downtime and your PVCs are in healthy state, you may follow this procedure:
- Delete your existing `MariaDB`, this will leave your PVCs intact.
- Create your `MariaDB` again, this will trigger a Galera recovery process as described in [this section](#bootstrap-galera-cluster-from-existing-pvcs).

As a last resource, you can always delete the PVCs and bootstrap a new `MariaDB` from a backup as documented [here](./logical_backup.md#bootstrap-new-mariadb-instances).
 
#### Permission denied writing Galera configuration

This error occurs when the user that runs the container does not have enough privileges to write in `/etc/mysql/mariadb.conf.d`:

```bash
 Error writing Galera config: open /etc/mysql/mariadb.conf.d/0-galera.cnf: permission denied
```

To mitigate this, by default, the operator sets the following `securityContext` in the `MariaDB`'s `StatefulSet` :

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mariadb-galera
spec:
  securityContext:
    fsGroup: 999
    runAsGroup: 999
    runAsNonRoot: true
    runAsUser: 999
```

This enables the `CSIDriver` and the kubelet to recursively set the ownership ofr the `/etc/mysql/mariadb.conf.d` folder to the group `999`, which is the one expected by MariaDB. It is important to note that not all the `CSIDrivers` implementations support this feature, see the [CSIDriver documentation](https://kubernetes-csi.github.io/docs/support-fsgroup.html) for further information.

#### Unauthorized error disabling bootstrap

```bash
Error reconciling Galera: error disabling bootstrap in Pod 0: unauthorized
```
This situation occurs when the `mariadb-operator` credentials passed to the `agent` as authentication are either invalid or the `agent` is unable to verify them. To confirm this, ensure that both the `mariadb-operator` and the `MariaDB` `ServiceAccounts` are able to create `TokenReview` objects:

```bash
kubectl auth can-i --list --as=system:serviceaccount:default:mariadb-operator | grep tokenreview
tokenreviews.authentication.k8s.io              []                                    []               [create]

kubectl auth can-i --list --as=system:serviceaccount:default:mariadb-galera | grep tokenreview
tokenreviews.authentication.k8s.io              []                                    []               [create]
```

If that's not the case, check that the following `ClusterRole` and `ClusterRoleBindings` are available in your cluster:
```bash
kubectl get clusterrole system:auth-delegator
NAME                    CREATED AT
system:auth-delegator   2023-08-03T19:12:37Z

kubectl get clusterrolebinding | grep mariadb | grep auth-delegator
mariadb-galera:auth-delegator                     ClusterRole/system:auth-delegator                                                  108m
mariadb-operator:auth-delegator                        ClusterRole/system:auth-delegator                                                  112m
```
`mariadb-operator:auth-delegator` is the `ClusterRoleBinding` bound to the `mariadb-operator` `ServiceAccount` which is created by the helm chart, so you can re-install the helm release in order to recreate it:

```bash
 helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator
```

`mariadb-galera:auth-delegator` is the `ClusterRoleBinding` bound to the `mariadb-galera` `ServiceAccount` which is created on the flight by the operator as part of the reconciliation logic. You may check the `mariadb-operator` logs to see if there are any issues reconciling it.

Bear in mind that `ClusterRoleBindings` are cluster-wide resources that are not garbage collected when the `MariaDB` owner object is deleted, which means that creating and deleting `MariaDBs` could leave leftovers in your cluster. These leftovers can lead to RBAC misconfigurations, as the `ClusterRoleBinding` might not be pointing to the right `ServiceAccount`. To overcome this, you can override the `ClusterRoleBinding` name setting the `spec.galera.agent.kubernetesAuth.authDelegatorRoleName` field.

#### Timeout waiting for Pod to be Synced

```bash
Timeout waiting for Pod 'mariadb-galera-2' to be Synced
```
This error appears in the `mariadb-operator` logs when a `Pod` is in non synced state for a duration exceeding the `spec.galera.recovery.podRecoveryTimeout`. Just after, the operator will restart the `Pod`.

Increase this timeout if you consider that your `Pod` may take longer to recover.

#### Galera cluster bootstrap timed out

```bash
Galera cluster bootstrap timed out. Resetting recovery status
```
This is error is returned by the `mariadb-operator` after exceeding the `spec.galera.recovery.clusterBootstrapTimeout` when recovering the cluster. At this point, the operator will reset the recovered sequence numbers and start again from a clean state.

Increase this timeout if you consider that your Galera cluster may take longer to recover.

### GitHub Issues

Here it is a list of Galera-related issues reported by `mariadb-operator` users which might shed some light in your investigation:
- https://github.com/mariadb-operator/mariadb-operator/issues?q=label%3Agalera+

## Reference
- [API reference](./api_reference.md)
- [Example suite](../examples/)
