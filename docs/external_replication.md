# External replication

`mariadb-operator` supports replication from an external MariaDB instances i.e running outside of the Kubernetes cluster where the operator runs. This feature allow us to create a cluster of replicas of an external MariaDB.

## Table of contents
<!-- toc -->
- [`ExternalMariaDB` configuration](#externalmariadb-configuration)
- [`MariaDB` configuration](#mariadb-configuration)
- [Backup considerations](#backup-considerations)
- [Replication self-repair behaviour](#replication-self-repair-behaviour)
- [Service considerations](#services-considerations)
<!-- /toc -->

## `ExternalMariaDB` configuration

To setup the external replication first we need add our source MariaDB as an `ExternalMariaDB`:
```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: ExternalMariaDB
metadata:
  name: external-mariadb
spec:
  host: mariadb.example.com
  port: 3306
  username: root
  passwordSecretKeyRef:
    name: mariadb
    key: password
  connection:
    secretName: external-mariadb
    healthCheck:
      interval: 5s
```


## MariaDB configuration

With the `ExternalMariaDB`created, we just need to define a regular MariaDB object with the replication enabled
and using the `replicaFromExternal` property to point it to our external database. 
With the `serverIdOffset`(optional) we can also set a specific `serverId` offset value to avoid conflicting 
with another replicas. 


```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: external-replicas
spec:
  storage:
    size: 10Gi
  replicas: 3
  replication:
    enabled: true
    replicaFromExternal:
      mariaDbRef:
        name: external-mariadb
        kind: ExternalMariaDB
      serverIdOffset: 30
  service:
    type: ClusterIP
  primaryService:
    type: ClusterIP
  secondaryService:
    type: ClusterIP
```
When applied it will create 3 new replicas from the external database. The operator will create a new backup 
and restore it on each pod and will configure the replication.

## Backup considerations
* The backup storage size will same as the storage size defined for the replicas.
* The backup expire date will be aligned with the `binlog` retention period on source database. 
* If you need to add more replicas(by increasing replica number), or need to re-build a replica the operator will always check if a valid backup is available before taking a new one.
* The backup will not include users to avoid privileges issues and conflicts, mainly, with the `root` user. Use the regular `User` and `Grant` object to manage the user and privileges on the replicas.

## Replication self-repair behaviour
* In case of a non-permanent replication issues the operator will try restart the replication.
* In case of a permanent replication issues, like SQL conflicts or missing binlogs, the operator will destroy the Pod and PVC to create a fresh new Pod (One per time, to avoid service disruption).

## Services considerations
* Service: This service will send connection to any Pod, regardless of their replication status.
* PrimaryService: Despite there is no real primary node on that setup, this was kept to provide a way to always send connections for the same Pod as it could be required for some applications. 
* SecondaryService: Send connections to all Pods with on `ReplicationStateSlave` or `ReplicationStateSlaveBroken` status. No connections will be sent to Pod with the
`ReplicationStateSlavePermanentBroken` status.
