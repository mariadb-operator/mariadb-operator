# MaxScale

> [!WARNING]  
> This documentation applies to `mariadb-operator` version >= v0.0.25

MaxScale is a sophisticated database proxy, router, and load balancer designed specifically for MariaDB. It provides a range of features that ensure optimal high availability:
- Query based routing: Transparently route write queries to the primary nodes and read queries to the replica nodes.
- Connection based routing: Load balance connection between multiple servers.
- Automatic primary failover based on MariaDB internals.
- Replay pending transactions when a server goes down.

To better understand what MaxScale is capable of you may check the [product page](https://mariadb.com/docs/server/products/mariadb-maxscale/) and the [documentation](https://mariadb.com/kb/en/maxscale/). 

## MaxScale resources

Prior to configuring MaxScale within Kubernetes, it's essential to have a basic understanding of the resources managed through its API.

#### Servers

A server defines the backend database servers that MaxScale forwards traffic to. For more detailed information, please consult the [server reference](https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#server).

#### Monitors

A monitor is agent that queries the state of the servers and makes it available to the services in order to route traffic based on it. For more detailed information, please consult the [monitor reference](https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#monitor).

Depending on which highly available configuration your servers have, you will need to choose betweeen the following modules:
- [Galera Monitor](https://mariadb.com/kb/en/mariadb-maxscale-2308-galera-monitor/): Detects whether servers are part of the cluster, ensuring synchronization among them, and assigning primary and replica roles as needed.
- [MariaDB Monitor](https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-monitor/): Probes the state of the cluster, assigns roles to the servers, and executes failover, switchover, and rejoin operations as necessary.
#### Services

A service defines how the traffic is routed to the servers based on a routing algorithm that takes into account the state of the servers and its role. For more detailed information, please consult the [service reference](https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#service).

Depending on your requirements to route traffic, you may choose between the following routers:
- [Readwritesplit](https://mariadb.com/kb/en/mariadb-maxscale-2308-readwritesplit/): Route write queries to the primary server and read queries to the replica servers:
- [Readconnroute](https://mariadb.com/kb/en/mariadb-maxscale-2308-readconnroute/): Load balance connections between multiple servers.

#### Listeners

A listener specifies a port where MaxScale listens for incoming connections. It is associated with a service that handles the requests received on that port. For more detailed information, please consult the [listener reference](https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#listener).

## `MaxScale` CR

The minimal spec you need to provision a MaxScale instance is just a reference to a `MariaDB` resource, like in this [example](../examples/manifests/mariadb_v1alpha1_maxscale.yaml):

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
  mariaDbRef:
    name: mariadb-galera
```

This will provision a new `StatefulSet` for running MaxScale and configure the servers specified by the `MariaDB` resource. Refer to the [Server configuration](#server-configuration) section if you want to manually configure the MariaDB servers.

The rest of the configuration uses reasonable [defaults](#defaults) set automatically by the operator. If you need a more fine grained configuration, you can provide this values yourself, see Galera [example](../examples/manifests/mariadb_v1alpha1_maxscale_galera.yaml):

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
...
  mariaDbRef:
    name: mariadb-galera

  services:
    - name: rw-router
      router: readwritesplit
      params:
        transaction_replay: "true"
        transaction_replay_attempts: "10"
        transaction_replay_timeout: "5s"
        max_slave_connections: "255"
        max_replication_lag: "3s"
        master_accept_reads: "true"
      listener:
        port: 3306
        protocol: MariaDBProtocol
        params:
          connection_metadata: "tx_isolation=auto"
    - name: rconn-master-router
      router: readconnroute
      params:
        router_options: "master"
        max_replication_lag: "3s"
        master_accept_reads: "true"
      listener:
        port: 3307
    - name: rconn-slave-router
      router: readconnroute
      params:
        router_options: "slave"
        max_replication_lag: "3s"
      listener:
        port: 3308

  monitor:
    interval: 2s
    cooperativeMonitoring: majority_of_all
    params:
      disable_master_failback: "false"
      available_when_donor: "false"
      disable_master_role_setting: "false"

  kubernetesService:
    type: LoadBalancer
    annotations:
      metallb.universe.tf/loadBalancerIPs: 172.18.0.224
```

As you can see, the [MaxScale resources](#maxscale-resources) we previously mentioned have a counterpart resource in the `MaxScale` CR. 

The previous example configured a `MaxScale` for a Galera cluster, but you may also configure `MaxScale` with a `MariaDB` that uses replication. It is important to note that the monitor module is automatically infered by the operator based on the `MariaDB` reference you provided, however, its parameters are specific to each monitor module. See the replication [example](../examples/manifests/mariadb_v1alpha1_maxscale_replication.yaml):


```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-repl
spec:
...
  mariaDbRef:
    name: mariadb-repl

  services:
    - name: rw-router
      router: readwritesplit
      params:
        transaction_replay: "true"
        transaction_replay_attempts: "10"
        transaction_replay_timeout: "5s"
        max_slave_connections: "255"
        max_replication_lag: "3s"
        master_accept_reads: "true"
      listener:
        port: 3306
        protocol: MariaDBProtocol
        params:
          connection_metadata: "tx_isolation=auto"
    - name: rconn-master-router
      router: readconnroute
      params:
        router_options: "master"
        max_replication_lag: "3s"
        master_accept_reads: "true"
      listener:
        port: 3307
    - name: rconn-slave-router
      router: readconnroute
      params:
        router_options: "slave"
        max_replication_lag: "3s"
      listener:
        port: 3308

  monitor:
    interval: 2s
    cooperativeMonitoring: majority_of_all
    params:
      auto_failover: "true"
      auto_rejoin: "true"
      switchover_on_low_disk_space: "true"

  kubernetesService:
    type: LoadBalancer
    annotations:
      metallb.universe.tf/loadBalancerIPs: 172.18.0.214
```

Once you have provisioned the `MaxScale` resource, you also need to set a reference in the `MariaDB` resource. This is explained in the [MariaDB CR](#mariadb-cr) section.

Refer to the [Reference](#reference) section for further detail.

## `MariaDB` CR

After having provisioned a `MaxScale` resource as described in the [MaxScale CR](#mariadb-cr) section, you also need to make the `MariaDB` resource aware of this by setting a `spec.maxScaleRef`. By doing so, high availability tasks such the primary failover will be delegated to `MaxScale`, see the following [example](../examples/manifests/mariadb_v1alpha1_mariadb_galera_maxscale.yaml):

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
...
  maxScaleRef:
    name: maxscale-galera

  galera:
    enabled: true
```

Refer to the [Reference](#reference) section for further detail.

## `MariaDB` + `MaxScale` CRs

In order to simplify the setup described in the [MaxScale CR](#mariadb-cr) and [MariaDB CR](#mariadb-cr) sections, you can provision a `MaxScale` to be used with `MariaDB` in just one resource, take a look at this [example](../examples/manifests/mariadb_v1alpha1_mariadb_galera_maxscale.yaml):

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
...
  maxScale:
    enabled: true

    kubernetesService:
      type: LoadBalancer
      annotations:
        metallb.universe.tf/loadBalancerIPs: 172.18.0.229

  galera:
    enabled: true
```
This will automatically setup the references between `MariaDB` and `MaxScale` and [default](#defaults) the rest of the fields as described in previous sections.

Refer to the [Reference](#reference) section for further detail.

## Defaults

`mariadb-operator` aims to provide highly configurable CRs, but at the same maximize its usability by providing reasonable defaults. In the case of `MaxScale`, the following defaulting logic is applied:
- `spec.servers` are infered from `spec.mariaDbRef` 
- `spec.monitor.module` is infered from the `spec.mariaDbRef`
- If `spec.services` is not provided, the following are configured by default
  - `readwritesplit` service on port `3306`
  - `readconnroute` service pointing to the primary node on port `3307`
  - `readconnroute` service pointing to the replica nodes on port `3308`

## Server configuration

As an alternative to provide a reference to a `MariaDB` via `spec.mariaDbRef`, you can also specify the servers manually, like in this [example](../examples/manifests/mariadb_v1alpha1_maxscale_full.yaml):

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
...
  servers:
    - name: mariadb-0
      address: mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local
    - name: mariadb-1
      address: mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local
    - name: mariadb-2
      address: mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local
```

As you could see, you can refer to a in-cluser MariaDB server by providing the DNS names of the `MariaDB` `Pods` as server addresses. In addition, you can also refer to external MariaDB instances running outside of the Kubernetes cluster where `mariadb-operator` was deployed, see this [example](../examples/manifests/mariadb_v1alpha1_maxscale_external.yaml):

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
...
  servers:
    - name: mariadb-0
      address: 172.18.0.140
      port: 3306
    - name: mariadb-1
      address: 172.18.0.141
    - name: mariadb-2
      address: 172.18.0.142

  monitor:
    name: mariadb-monitor
    module: galeramon
    interval: 2s
    cooperativeMonitoring: majority_of_all
    params:
      disable_master_failback: "false"
      available_when_donor: "false"
      disable_master_role_setting: "false"

  auth:
    adminUsername: mariadb-operator
    adminPasswordSecretKeyRef:
      name: maxscale
      key: password
    clientUsername: maxscale-client
    clientPasswordSecretKeyRef:
      name: maxscale
      key: password
    serverUsername: maxscale-server
    serverPasswordSecretKeyRef:
      name: maxscale
      key: password
    monitorUsername: maxscale-monitor
    monitorPasswordSecretKeyRef:
      name: maxscale
      key: password
    syncUsername: maxscale-sync
    syncPasswordSecretKeyRef:
      name: maxscale
      key: password
```

⚠️ Pointing to external MariaDBs has a some limitations ⚠️. Since the operator doesn't have a reference to a `MariaDB` resource (`spec.mariaDbRef`), it will be unable to perform the following actions:
- Infer the monitor module (`spec.monitor.module`), so it will need to be provided by the user.
- Autogenerate authentication credentials (`spec.auth`), so they will need to be provided by the user. See [Authentication](#authentication) section. 

## Server maintenance

You can put servers in maintenance mode by setting `maintenance = true`, as this [example](../examples/manifests/mariadb_v1alpha1_maxscale_full.yaml) shows:

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
...
  servers:
    - name: mariadb-0
      address: mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local
      port: 3306
      protocol: MariaDBBackend
      maintenance: false
```

Maintenance mode prevents MaxScale from routing traffic to the server and also excludes it from being elected as the new primary during failover events.

## Configuration

Like MariaDB, MaxScale allows you to provide global configuration parameters in a `maxscale.conf` file. You don't need to provide this config file directly, but instead you can use the `spec.config.params` to instruct the operator to create the `maxscale.conf`, as this [example](../examples/manifests/mariadb_v1alpha1_maxscale_full.yaml) shows:

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
...
  config:
    params:
      log_info: "true"
    volumeClaimTemplate:
      resources:
        requests:
          storage: 100Mi
      accessModes:
        - ReadWriteOnce
```

Both this static configuration and the resources created by the operator using the [MaxScale API](#maxscale-api) are stored under a volume provisioned by the `spec.config.volumeClaimTemplate`.

Refer to the [MaxScale reference](https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/) to provide static configuration.

## Authentication

MaxScale requires authentication with differents levels of permissions for the following components/actors:
- Admin REST API consumed by `mariadb-operator`
- Clients connecting to MaxScale
- MaxScale connecting to MariaDB servers
- MaxScale monitor conneccting to MariaDB servers
- MaxScale configuration sync to connect to MariaDB servers. See [High availability](#high-availability) section.

By default, `mariadb-operator` autogenerates this credentials when `spec.mariaDbRef` is set and `spec.auth.generate = true`, but you are still able to provide your own, as this [example](../examples/manifests/mariadb_v1alpha1_maxscale_full.yaml) shows:

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
...
  auth:
    generate: true
    adminUsername: mariadb-operator
    adminPasswordSecretKeyRef:
      name: maxscale
      key: password
    deleteDefaultAdmin: true
    clientUsername: maxscale-client
    clientPasswordSecretKeyRef:
      name: maxscale
      key: password
    clientMaxConnections: 90
    serverUsername: maxscale-server
    serverPasswordSecretKeyRef:
      name: maxscale
      key: password
    serverMaxConnections: 90 
    monitorUsername: maxscale-monitor
    monitorPasswordSecretKeyRef:
      name: maxscale
      key: password
    monitorMaxConnections: 90 
    syncUsername: maxscale-sync
    syncPasswordSecretKeyRef:
      name: maxscale
      key: password
    syncMaxConnections: 90
```

As you could see, you are also able to limit the number of connections for each component/actor. Bear in mind that, when running in [High availability](#high-availability), you may need to increase this number, as more MaxScale instances implies more connections.

## Connection

You can leverage the `Connection` resource to automatically configure connection strings in `Secret` resources that your applications can mount, see this [example](../examples/manifests/mariadb_v1alpha1_connection_maxscale.yaml):

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: Connection
metadata:
  name: connection-maxscale
spec:
  maxScaleRef:
    name: maxscale-galera
  username: maxscale-galera-client
  passwordSecretKeyRef:
    name: maxscale-galera-client
    key: password
  secretName: conn-mxs
```

Alternatively, you can also provide a connection template to your `MaxScale` resource, see this [example](../examples/manifests/mariadb_v1alpha1_maxscale.yaml):

```yaml
apiVersion: mariadb.mmontes.io/v1alpha1
kind: MaxScale
metadata:
  name: maxscale-galera
spec:
...  
  connection:
    secretName: mxs-galera-conn
    port: 3306
```

## High availability

## Status

## MaxScale GUI

## MaxScale API

`mariadb-operator`interacts with the [MaxScale REST API](https://mariadb.com/kb/en/mariadb-maxscale-23-08-rest-api/) to reconcile the specification provided by the user, considering both the MaxScale status retrieved from the API and the provided spec.

[<img src="https://run.pstmn.io/button.svg" alt="Run In Postman" style="width: 128px; height: 32px;">](https://www.postman.com/mariadb-operator/workspace/mariadb-operator/collection/9776-74dfd54a-2b2b-451f-95ab-006e1d9d9998?action=share&creator=9776&active-environment=9776-a841398f-204a-48c8-ac04-6f6c3bb1d268)

## Reference
- [API reference](./API_REFERENCE.md)
- [Example suite](../examples/)