# High availability via Galera

The `mariadb-operator` provides cloud native support for provisioning and operating multi-master MariaDB clusters using Galera. This setup enables the ability to perform both read and write operations on all nodes, enhancing availability and allowing scalability across multiple nodes.

In certain circumstances, it could be the case that all the nodes of your cluster go down, something that Galera is not able to recover by itself, and it requires manual action to bring the cluster up again, as documented in the [Galera documentation](https://galeracluster.com/library/documentation/crash-recovery.html). Luckly, `mariadb-operator` has you covered and it encapsulates this operational expertise in the `MariaDB` CRD. You just need to declaratively specify `spec.galera`, as explained in more detail [later in this guide](#configuration).

To accomplish this, after the MariaDB cluster has been provisioned, `mariadb-operator` will regularly monitor the cluster's status to make sure it is healthy. If any issues are detected, the operator will initiate the [recovery process](https://galeracluster.com/library/documentation/crash-recovery.html) to restore the cluster to a healthy state. During this process, the operator will set status conditions in the `MariaDB` and emit `Events` so you have a better understanding of the recovery progress and the underlying activities being performed. For example, you may want to know which `Pods` were out of sync to further investigate infrastructure-related issues (i.e. networking, storage...) on the nodes where these `Pods` were scheduled.

## Components

To be able to effectively provision and recover MariaDB Galera clusters, the following components were introduced to co-operate with `mariadb-operator`:
- **[🍼 init](https://github.com/mariadb-operator/init)**: Init container that dynamically provisions the Galera configuration file before the MariaDB container starts. Guarantees ordered deployment of `Pods` even if `spec.podManagementPolicy = Parallel` is set on the MariaDB `StatefulSet`, something crucial for performing the Galera recovery, as the operator needs to restart `Pods` independently.
- **[🤖 agent](https://github.com/mariadb-operator/agent)**: Sidecar agent that exposes the Galera state ([`grastate.dat`](https://galeracluster.com/2016/11/introducing-the-safe-to-bootstrap-feature-in-galera-cluster/)) via HTTP and allows one to remotely bootstrap and recover the Galera cluster. For security reasons, it has authentication based on Kubernetes service accounts; this way only the `mariadb-operator` is able to call the agent.

## Configuration

The easiest way to get a MariaDB Galera cluster up and running is setting `spec.galera.enabled = true`, like in this [example](../examples/manifests/mariadb_v1alpha1_mariadb_galera_minimal.yaml):

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
...
  galera:
    enabled: true
...
```

This relies on sensible defaults set by either the operator or the webhook, which may not be suitable for your Kubernetes cluster. This can be solved by overriding the defaults, as in this other [example](../examples/manifests/mariadb_v1alpha1_mariadb_galera.yaml), so you have fine-grained control over the Galera configuration:


```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
...
  galera:
    enabled: true
    primary:
      podIndex: 0
      automaticFailover: true
    sst: mariabackup
    replicaThreads: 1
    agent:
      image:
        repository: ghcr.io/mariadb-operator/agent
        tag: "v0.0.2"
        pullPolicy: IfNotPresent
      port: 5555
      kubernetesAuth:
        enabled: true
      gracefulShutdownTimeout: 5s
    recovery:
      enabled: true
      clusterHealthyTimeout: 3m
      clusterBootstrapTimeout: 10m
      podRecoveryTimeout: 5m
      podSyncTimeout: 5m
    initContainer:
      image:
        repository: ghcr.io/mariadb-operator/init
        tag: "v0.0.5"
        pullPolicy: IfNotPresent
    volumeClaimTemplate:
      resources:
        requests:
          storage: 300Mi
      accessModes:
        - ReadWriteOnce
...
```

Refer to the [API Reference](#api-reference) below to better understand the purpose of each field.

## API Reference
- [Go API pkg](https://pkg.go.dev/github.com/mariadb-operator/mariadb-operator@v0.0.16/api/v1alpha1#Galera)
- [Code](../api/v1alpha1/mariadb_galera_types.go)
- **`kubectl explain`**
```bash
kubectl explain mariadb.spec.galera
...
FIELDS:
...
   recovery     <Object>
     GaleraRecovery is the recovery process performed by the operator whenever
     the Galera cluster is not healthy. More info:
     https://galeracluster.com/library/documentation/crash-recovery.html.

   replicaThreads       <integer>
     ReplicaThreads is the number of replica threads used to apply Galera write
     sets in parallel. More info:
     https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_slave_threads.

   sst  <string>
     SST is the Snapshot State Transfer used when new Pods join the cluster.
     More info: https://galeracluster.com/library/documentation/sst.html.

   volumeClaimTemplate  <Object>
     VolumeClaimTemplate is a template for the PVC that will contain the Galera
     configuration files shared between the InitContainer, Agent and MariaDB.

kubectl explain mariadb.spec.galera.recovery
...
FIELDS:
...
  clusterBootstrapTimeout      <string>
    ClusterBootstrapTimeout is the time limit for bootstrapping a cluster. Once
    this timeout is reached, the Galera recovery state is reset and a new
    cluster bootstrap will be attempted.

  clusterHealthyTimeout        <string>
    ClusterHealthyTimeout represents the duration at which a Galera cluster,
    that consistently failed health checks, is considered unhealthy, and
    consequently the Galera recovery process will be initiated by the operator.

  podRecoveryTimeout   <string>
    PodRecoveryTimeout is the time limit for executing the recovery sequence
    within a Pod. This process includes enabling the recovery mode in the
    Galera configuration file, restarting the Pod and retrieving the sequence
    from a log file.

  podSyncTimeout       <string>
    PodSyncTimeout is the time limit we give to a Pod to reach the Sync state.
    Once this timeout is reached, the Pod is restarted.
```

## Quickstart

Let's see how `mariadb-operator`🦭 and Galera play together! First of all, install the following configuration manifests that will be referenced by the CRDs further:
```bash
kubectl apply -f examples/manifests/config
```
Next, you can proceed with the installation of a `MariaDB` instance with Galera support:
```bash
kubectl apply -f examples/manifests/mariadb_v1alpha1_mariadb_galera.yaml
```
```bash
kubectl get mariadbs
NAME             READY   STATUS    PRIMARY POD   AGE
mariadb-galera   True    Running   All           48m

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
mariadb-galera   3/3     58m   mariadb,agent   mariadb:11.0.3,ghcr.io/mariadb-operator/agent:v0.0.2

kubectl get pods -o wide
NAME                                        READY   STATUS    RESTARTS   AGE   IP           NODE          NOMINATED NODE   READINESS GATES
mariadb-galera-0                            2/2     Running   0          58m   10.244.2.4   mdb-worker3   <none>           <none>
mariadb-galera-1                            2/2     Running   0          58m   10.244.1.9   mdb-worker2   <none>           <none>
mariadb-galera-2                            2/2     Running   0          58m   10.244.5.4   mdb-worker4   <none>           <none>
```
Up and running 🚀. All right, please fasten your seatbelts and let's proceed with simulating a Galera cluster failure 💥:
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
Finally, the `MariaDB` will become `Ready` again, and your Galera cluster will be back to life! 🦭🎉:
```bash
kubectl get mariadb mariadb-galera -o jsonpath="{.status.conditions[?(@.type=='GaleraReady')]}"
{"lastTransitionTime":"2023-07-13T19:27:51Z","message":"Galera ready","reason":"GaleraReady","status":"True","type":"GaleraReady"}

kubectl get mariadb mariadb-galera
NAME             READY   STATUS    PRIMARY POD   AGE
mariadb-galera   True    Running   All           82m
```

To conclude, it's important to note that the Galera functionallity is 100% compatible with the rest of `mariadb-operator` constructs: `Backup`, `Restore`, `Connection`... refer to the [main quickstart guide](../README.md#quickstart) for more detail.

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

#### Permission denied writing Galera configuration

```bash
 Error writing Galera config: open /etc/mysql/mariadb.conf.d/0-galera.cnf: permission denied
```
This error is returned by the `init` container when it is unable to write the configuration file in the filesystem backed by the PVC. In particular, this has been raised by users using longhorn and rook as a storage provider, which in some cases rely on root privileges for writing in the PVC:
- https://github.com/longhorn/longhorn/issues/3549

The remediation is running as root or match the user expected by the storage provider to be able to write in the PVC:

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  podSecurityContext:
    runAsUser: 0
...
```

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

Here it is a list of GitHub issues reported by `mariadb-operator` users which might shed some light in your investigation:
- https://github.com/mariadb-operator/mariadb-operator/issues?q=is%3Aclosed+label%3Agalera-troubleshoot+