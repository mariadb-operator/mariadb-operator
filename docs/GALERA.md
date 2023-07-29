## ✨ High availability via Galera

The `mariadb-operator` provides cloud native support for provisioning and operating multi-master MariaDB clusters using Galera. This setup enables the ability to perform both read and write operations on all nodes, enhancing availability and allowing scalability across multiple nodes.

In certain circumstances, it could be the case that all the nodes of your cluster go down, something that Galera is not able to recover by itself, and it requires manual action to bring the cluster up again, as documented in the [Galera documentation](https://galeracluster.com/library/documentation/crash-recovery.html). Luckly, `mariadb-operator` has you covered and it encapsulates this operational expertise in the `MariaDB` CRD. You just need to declaratively specify `spec.galera`, as explained in more detail [later in this guide](#configuration).

To accomplish this, after the MariaDB cluster has been provisioned, `mariadb-operator` will regularly monitor the cluster's status to make sure it is healthy. If any issues are detected, the operator will initiate the [recovery process](https://galeracluster.com/library/documentation/crash-recovery.html) to restore the cluster to a healthy state. During this process, the operator will set status conditions in the `MariaDB` and emit `Events` so you have a better understanding of the recovery progress and the underlying activities being performed. For example, you may want to know which `Pods` were out of sync to further investigate infrastructure-related issues (i.e. networking, storage...) on the nodes where these `Pods` were scheduled.

### Components

To be able to effectively provision and recover MariaDB Galera clusters, the following components were introduced to co-operate with `mariadb-operator`:
- **[🍼 init](https://github.com/mariadb-operator/init)**: Init container that dynamically provisions the Galera configuration file before the MariaDB container starts. Guarantees ordered deployment of `Pods` even if `spec.podManagementPolicy = Parallel` is set on the MariaDB `StatefulSet`, something crucial for performing the Galera recovery, as the operator needs to restart `Pods` independently.
- **[🤖 agent](https://github.com/mariadb-operator/agent)**: Sidecar agent that exposes the Galera state ([`grastate.dat`](https://galeracluster.com/2016/11/introducing-the-safe-to-bootstrap-feature-in-galera-cluster/)) via HTTP and allows one to remotely bootstrap and recover the Galera cluster. For security reasons, it has authentication based on Kubernetes service accounts; this way only the `mariadb-operator` is able to call the agent.

### Configuration

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
        authDelegatorRoleName: mariadb-galera-auth
      gracefulShutdownTimeout: 5s
    recovery:
      enabled: true
      clusterHealthyTimeout: 1m
      clusterBootstrapTimeout: 5m
      podRecoveryTimeout: 3m
      podSyncTimeout: 3m
    initContainer:
      image:
        repository: ghcr.io/mariadb-operator/init
        tag: "v0.0.5"
        pullPolicy: IfNotPresent
    volumeClaimTemplate:
      spec:
        resources:
          requests:
            storage: 50Mi
        accessModes:
          - ReadWriteOnce
...
```

Refer to the [API Reference](#api-reference) below to better understand the purpose of each field.

### API Reference
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

### Quickstart

Let's see how `mariadb-operator`🦭 and Galera✨ play together! First of all, install the following configuration manifests that will be referenced by the CRDs further:
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
mariadb-galera   3/3     58m   mariadb,agent   mariadb:11.0.2,ghcr.io/mariadb-operator/agent:v0.0.2

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
NAME             READY   STATUS             PRIMARY POD   AGE
mariadb-galera   False   Galera not ready   All           67m

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
