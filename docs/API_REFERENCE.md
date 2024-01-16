# API Reference

## Packages
- [mariadb.mmontes.io/v1alpha1](#mariadbmmontesiov1alpha1)


## mariadb.mmontes.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the v1alpha1 API group

### Resource Types
- [Backup](#backup)
- [Connection](#connection)
- [Database](#database)
- [Grant](#grant)
- [MariaDB](#mariadb)
- [MaxScale](#maxscale)
- [Restore](#restore)
- [SqlJob](#sqljob)
- [User](#user)



#### Backup



Backup is the Schema for the backups API. It is used to define backup jobs and its storage.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `mariadb.mmontes.io/v1alpha1`
| `kind` _string_ | `Backup`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[BackupSpec](#backupspec)_ |  |


#### BackupSpec



BackupSpec defines the desired state of Backup

_Appears in:_
- [Backup](#backup)

| Field | Description |
| --- | --- |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |
| `storage` _[BackupStorage](#backupstorage)_ | Storage to be used in the Backup. |
| `args` _string array_ | Args to be used in the Backup container. |
| `schedule` _[Schedule](#schedule)_ | Schedule defines when the Backup will be taken. |
| `maxRetention` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | MaxRetention defines the retention policy for backups. Old backups will be cleaned up by the Backup Job. It defaults to 30 days. |
| `logLevel` _string_ | LogLevel to be used n the Backup Job. It defaults to 'info'. |
| `backoffLimit` _integer_ | BackoffLimit defines the maximum number of attempts to successfully take a Backup. |
| `restartPolicy` _[RestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#restartpolicy-v1-core)_ | RestartPolicy to be added to the Backup Pod. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ | Affinity to be used in the Backup Pod. |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Backup Pod. |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Backup Pod. |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |


#### BackupStorage



BackupStorage defines the storage for a Backup.

_Appears in:_
- [BackupSpec](#backupspec)

| Field | Description |
| --- | --- |
| `s3` _[S3](#s3)_ | S3 defines the configuration to store backups in a S3 compatible storage. |
| `persistentVolumeClaim` _[PersistentVolumeClaimSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimspec-v1-core)_ | PersistentVolumeClaim is a Kubernetes PVC specification. |
| `volume` _[VolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumesource-v1-core)_ | Volume is a Kubernetes volume specification. |


#### Connection



Connection is the Schema for the connections API. It is used to configure connection strings for the applications connecting to MariaDB.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `mariadb.mmontes.io/v1alpha1`
| `kind` _string_ | `Connection`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[ConnectionSpec](#connectionspec)_ |  |


#### ConnectionSpec



ConnectionSpec defines the desired state of Connection

_Appears in:_
- [Connection](#connection)

| Field | Description |
| --- | --- |
| `secretName` _string_ | SecretName to be used in the Connection. |
| `secretTemplate` _[SecretTemplate](#secrettemplate)_ | SecretTemplate to be used in the Connection. |
| `healthCheck` _[HealthCheck](#healthcheck)_ | HealthCheck to be used in the Connection. |
| `params` _object (keys:string, values:string)_ | Params to be used in the Connection. |
| `serviceName` _string_ | ServiceName to be used in the Connection. |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |
| `username` _string_ | Username to use for configuring the Connection. |
| `passwordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | PasswordSecretKeyRef is a reference to the password to use for configuring the Connection. |
| `database` _string_ | Database to use for configuring the Connection. |


#### ConnectionTemplate



ConnectionTemplate defines a template to customize Connection objects.

_Appears in:_
- [ConnectionSpec](#connectionspec)
- [MariaDBSpec](#mariadbspec)

| Field | Description |
| --- | --- |
| `secretName` _string_ | SecretName to be used in the Connection. |
| `secretTemplate` _[SecretTemplate](#secrettemplate)_ | SecretTemplate to be used in the Connection. |
| `healthCheck` _[HealthCheck](#healthcheck)_ | HealthCheck to be used in the Connection. |
| `params` _object (keys:string, values:string)_ | Params to be used in the Connection. |
| `serviceName` _string_ | ServiceName to be used in the Connection. |


#### Container



Container object definition.

_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)
- [PodTemplate](#podtemplate)

| Field | Description |
| --- | --- |
| `command` _string array_ | Command to be used in the Container. |
| `args` _string array_ | Args to be used in the Container. |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |
| `image` _string_ | Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`. |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |


#### ContainerTemplate



ContainerTemplate defines a template to configure Container objects.

_Appears in:_
- [Container](#container)
- [Exporter](#exporter)
- [GaleraAgent](#galeraagent)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `command` _string array_ | Command to be used in the Container. |
| `args` _string array_ | Args to be used in the Container. |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |


#### CooperativeMonitoring

_Underlying type:_ _string_

CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors. See: https://mariadb.com/docs/server/architecture/components/maxscale/monitors/mariadbmon/use-cooperative-locking-ha-maxscale-mariadb-monitor/

_Appears in:_
- [MaxScaleMonitor](#maxscalemonitor)



#### Database



Database is the Schema for the databases API. It is used to define a logical database as if you were running a 'CREATE DATABASE' statement.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `mariadb.mmontes.io/v1alpha1`
| `kind` _string_ | `Database`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[DatabaseSpec](#databasespec)_ |  |


#### DatabaseSpec



DatabaseSpec defines the desired state of Database

_Appears in:_
- [Database](#database)

| Field | Description |
| --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconcilizations. |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |
| `characterSet` _string_ | CharacterSet to use in the Database. |
| `collate` _string_ | CharacterSet to use in the Database. |
| `name` _string_ | Name overrides the default Database name provided by metadata.name. |


#### Exporter



Exporter defines a metrics exporter container.

_Appears in:_
- [Metrics](#metrics)

| Field | Description |
| --- | --- |
| `command` _string array_ | Command to be used in the Container. |
| `args` _string array_ | Args to be used in the Container. |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |
| `image` _string_ | Image name to be used as metrics exporter. The supported format is `<image>:<tag>`. Only mysqld-exporter >= v0.15.0 is supported: https://github.com/prometheus/mysqld_exporter |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |
| `port` _integer_ | Port where the exporter will be listening for connections. |


#### Galera



Galera allows you to enable multi-master HA via Galera in your MariaDB cluster.

_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description |
| --- | --- |
| `primary` _[PrimaryGalera](#primarygalera)_ | Primary is the Galera configuration for the primary node. |
| `sst` _[SST](#sst)_ | SST is the Snapshot State Transfer used when new Pods join the cluster. More info: https://galeracluster.com/library/documentation/sst.html. |
| `replicaThreads` _integer_ | ReplicaThreads is the number of replica threads used to apply Galera write sets in parallel. More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_slave_threads. |
| `agent` _[GaleraAgent](#galeraagent)_ | GaleraAgent is a sidecar agent that co-operates with mariadb-operator. More info: https://github.com/mariadb-operator/agent. |
| `recovery` _[GaleraRecovery](#galerarecovery)_ | GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy. More info: https://galeracluster.com/library/documentation/crash-recovery.html. |
| `initContainer` _[Container](#container)_ | InitContainer is an init container that co-operates with mariadb-operator. More info: https://github.com/mariadb-operator/init. |
| `volumeClaimTemplate` _[VolumeClaimTemplate](#volumeclaimtemplate)_ | VolumeClaimTemplate is a template for the PVC that will contain the Galera configuration files shared between the InitContainer, Agent and MariaDB. |
| `enabled` _boolean_ | Enabled is a flag to enable Galera. |


#### GaleraAgent



GaleraAgent is a sidecar agent that co-operates with mariadb-operator. More info: https://github.com/mariadb-operator/agent.

_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description |
| --- | --- |
| `command` _string array_ | Command to be used in the Container. |
| `args` _string array_ | Args to be used in the Container. |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |
| `image` _string_ | Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`. |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |
| `port` _integer_ | Port where the agent will be listening for connections. |
| `kubernetesAuth` _[KubernetesAuth](#kubernetesauth)_ | KubernetesAuth to be used by the agent container |
| `gracefulShutdownTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | GracefulShutdownTimeout is the time we give to the agent container in order to gracefully terminate in-flight requests. |


#### GaleraRecovery



GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy. More info: https://galeracluster.com/library/documentation/crash-recovery.html.

_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description |
| --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable GaleraRecovery. |
| `clusterHealthyTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | ClusterHealthyTimeout represents the duration at which a Galera cluster, that consistently failed health checks, is considered unhealthy, and consequently the Galera recovery process will be initiated by the operator. |
| `clusterBootstrapTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | ClusterBootstrapTimeout is the time limit for bootstrapping a cluster. Once this timeout is reached, the Galera recovery state is reset and a new cluster bootstrap will be attempted. |
| `podRecoveryTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | PodRecoveryTimeout is the time limit for executing the recovery sequence within a Pod. This process includes enabling the recovery mode in the Galera configuration file, restarting the Pod and retrieving the sequence from a log file. |
| `podSyncTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | PodSyncTimeout is the time limit we give to a Pod to reach the Sync state. Once this timeout is reached, the Pod is restarted. |


#### GaleraRecoveryBootstrap



GaleraRecoveryBootstrap indicates when and in which Pod the cluster bootstrap process has been performed.

_Appears in:_
- [GaleraRecoveryStatus](#galerarecoverystatus)

| Field | Description |
| --- | --- |
| `time` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ |  |
| `pod` _string_ |  |


#### GaleraSpec



GaleraSpec is the Galera desired state specification.

_Appears in:_
- [Galera](#galera)

| Field | Description |
| --- | --- |
| `primary` _[PrimaryGalera](#primarygalera)_ | Primary is the Galera configuration for the primary node. |
| `sst` _[SST](#sst)_ | SST is the Snapshot State Transfer used when new Pods join the cluster. More info: https://galeracluster.com/library/documentation/sst.html. |
| `replicaThreads` _integer_ | ReplicaThreads is the number of replica threads used to apply Galera write sets in parallel. More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_slave_threads. |
| `agent` _[GaleraAgent](#galeraagent)_ | GaleraAgent is a sidecar agent that co-operates with mariadb-operator. More info: https://github.com/mariadb-operator/agent. |
| `recovery` _[GaleraRecovery](#galerarecovery)_ | GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy. More info: https://galeracluster.com/library/documentation/crash-recovery.html. |
| `initContainer` _[Container](#container)_ | InitContainer is an init container that co-operates with mariadb-operator. More info: https://github.com/mariadb-operator/init. |
| `volumeClaimTemplate` _[VolumeClaimTemplate](#volumeclaimtemplate)_ | VolumeClaimTemplate is a template for the PVC that will contain the Galera configuration files shared between the InitContainer, Agent and MariaDB. |


#### Grant



Grant is the Schema for the grants API. It is used to define grants as if you were running a 'GRANT' statement.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `mariadb.mmontes.io/v1alpha1`
| `kind` _string_ | `Grant`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[GrantSpec](#grantspec)_ |  |


#### GrantSpec



GrantSpec defines the desired state of Grant

_Appears in:_
- [Grant](#grant)

| Field | Description |
| --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconcilizations. |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |
| `privileges` _string array_ | Privileges to use in the Grant. |
| `database` _string_ | Database to use in the Grant. |
| `table` _string_ | Table to use in the Grant. |
| `username` _string_ | Username to use in the Grant. |
| `host` _string_ | Host to use in the Grant. |
| `grantOption` _boolean_ | GrantOption to use in the Grant. |


#### Gtid

_Underlying type:_ _string_

Gtid indicates which Global Transaction ID should be used when connecting a replica to the master. See: https://mariadb.com/kb/en/gtid/#using-current_pos-vs-slave_pos.

_Appears in:_
- [ReplicaReplication](#replicareplication)



#### HealthCheck



HealthCheck defines intervals for performing health checks.

_Appears in:_
- [ConnectionSpec](#connectionspec)
- [ConnectionTemplate](#connectiontemplate)

| Field | Description |
| --- | --- |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Interval used to perform health checks. |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the intervañ used to perform health check retries. |


#### InheritMetadata



InheritMetadata defines the metadata to be inherited by children resources.

_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description |
| --- | --- |
| `labels` _object (keys:string, values:string)_ | Labels to be added to children resources. |
| `annotations` _object (keys:string, values:string)_ | Annotations to be added to children resources. |


#### KubernetesAuth



KubernetesAuth refers to the Kubernetes authentication mechanism utilized for establishing a connection from the operator to the agent. The agent validates the legitimacy of the service account token provided as an Authorization header by creating a TokenReview resource.

_Appears in:_
- [GaleraAgent](#galeraagent)

| Field | Description |
| --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable KubernetesAuth |
| `authDelegatorRoleName` _string_ | AuthDelegatorRoleName is the name of the ClusterRoleBinding that is associated with the "system:auth-delegator" ClusterRole. It is necessary for creating TokenReview objects in order for the agent to validate the service account token. |


#### MariaDB



MariaDB is the Schema for the mariadbs API. It is used to define MariaDB clusters.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `mariadb.mmontes.io/v1alpha1`
| `kind` _string_ | `MariaDB`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[MariaDBSpec](#mariadbspec)_ |  |


#### MariaDBRef



MariaDBRef is a reference to a MariaDB object.

_Appears in:_
- [BackupSpec](#backupspec)
- [ConnectionSpec](#connectionspec)
- [DatabaseSpec](#databasespec)
- [GrantSpec](#grantspec)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)
- [UserSpec](#userspec)

| Field | Description |
| --- | --- |
| `kind` _string_ | Kind of the referent. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `namespace` _string_ | Namespace of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/ |
| `name` _string_ | Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |
| `uid` _[UID](#uid)_ | UID of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids |
| `apiVersion` _string_ | API version of the referent. |
| `resourceVersion` _string_ | Specific resourceVersion to which this reference is made, if any. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency |
| `fieldPath` _string_ | If referring to a piece of an object instead of an entire object, this string should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2]. For example, if the object reference is to a container within a pod, this would take on a value like: "spec.containers{name}" (where "name" refers to the name of the container that triggered the event) or if no container name is specified "spec.containers[2]" (container with index 2 in this pod). This syntax is chosen only to have some well-defined way of referencing a part of an object. TODO: this design is not final and this field is subject to change in the future. |
| `waitForIt` _boolean_ | WaitForIt indicates whether the controller using this reference should wait for MariaDB to be ready. |


#### MariaDBSpec



MariaDBSpec defines the desired state of MariaDB

_Appears in:_
- [MariaDB](#mariadb)

| Field | Description |
| --- | --- |
| `command` _string array_ | Command to be used in the Container. |
| `args` _string array_ | Args to be used in the Container. |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |
| `initContainers` _[Container](#container) array_ | InitContainers to be used in the Pod. |
| `sidecarContainers` _[Container](#container) array_ | SidecarContainers to be used in the Pod. |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ | Affinity to be used in the Pod. |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ | Volumes to be used in the Pod. |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints to be used in the Pod. |
| `image` _string_ | Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`. Only MariaDB official images are supported. |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |
| `inheritMetadata` _[InheritMetadata](#inheritmetadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |
| `rootPasswordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | RootPasswordSecretKeyRef is a reference to a Secret key containing the root password. |
| `rootEmptyPassword` _boolean_ | RootEmptyPassword indicates if the root password should be empty. |
| `database` _string_ | Database is the database to be created on bootstrap. |
| `username` _string_ | Username is the username of the user to be created on bootstrap. |
| `passwordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | PasswordSecretKeyRef is a reference to the password of the initial user provided via a Secret. |
| `myCnf` _string_ | MyCnf allows to specify the my.cnf file mounted by Mariadb. |
| `myCnfConfigMapKeyRef` _[ConfigMapKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapkeyselector-v1-core)_ | MyCnfConfigMapKeyRef is a reference to the my.cnf config file provided via a ConfigMap. If not provided, it will be defaulted with reference to a ConfigMap with the contents of the MyCnf field. |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations to add to the Pods metadata. |
| `bootstrapFrom` _[RestoreSource](#restoresource)_ | BootstrapFrom defines a source to bootstrap from. |
| `metrics` _[Metrics](#metrics)_ | Metrics configures metrics and how to scrape them. |
| `replication` _[Replication](#replication)_ | Replication configures high availability via replication. |
| `galera` _[Galera](#galera)_ | Replication configures high availability via Galera. |
| `replicas` _integer_ | Replicas indicates the number of desired instances. |
| `port` _integer_ | Port where the instances will be listening for connections. |
| `ephemeralStorage` _boolean_ | EphemeralStorage indicates whether to use ephemeral storage for the instances. |
| `volumeClaimTemplate` _[VolumeClaimTemplate](#volumeclaimtemplate)_ | VolumeClaimTemplate provides a template to define the Pod PVCs. |
| `podDisruptionBudget` _[PodDisruptionBudget](#poddisruptionbudget)_ | PodDisruptionBudget defines the budget for replica availability. |
| `updateStrategy` _[StatefulSetUpdateStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#statefulsetupdatestrategy-v1-apps)_ | PodDisruptionBudget defines the update strategy for the StatefulSet object. |
| `service` _[ServiceTemplate](#servicetemplate)_ | Service defines templates to configure the general Service object. |
| `connection` _[ConnectionTemplate](#connectiontemplate)_ | Connection defines templates to configure the general Connection object. |
| `primaryService` _[ServiceTemplate](#servicetemplate)_ | PrimaryService defines templates to configure the primary Service object. |
| `primaryConnection` _[ConnectionTemplate](#connectiontemplate)_ | PrimaryConnection defines templates to configure the primary Connection object. |
| `secondaryService` _[ServiceTemplate](#servicetemplate)_ | SecondaryService defines templates to configure the secondary Service object. |
| `secondaryConnection` _[ConnectionTemplate](#connectiontemplate)_ | SecondaryConnection defines templates to configure the secondary Connection object. |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |


#### MaxScale



MaxScale is the Schema for the maxscales API



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `mariadb.mmontes.io/v1alpha1`
| `kind` _string_ | `MaxScale`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[MaxScaleSpec](#maxscalespec)_ |  |


#### MaxScaleAdmin



MaxScaleAdmin configures the admin REST API and GUI.

_Appears in:_
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `port` _integer_ | Port where the admin REST API will be exposed. |
| `guiEnabled` _boolean_ | GuiEnabled indicates whether the admin GUI should be enabled. |


#### MaxScaleAuth



MaxScaleAuth defines the credentials required for MaxScale to connect to MariaDB

_Appears in:_
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `adminUsername` _string_ | AdminUsername is an admin username to call the REST API. It is defaulted if not provided. |
| `adminPasswordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | AdminPasswordSecretKeyRef is Secret key reference to the admin password to call the REST API. It is defaulted if not provided. |
| `deleteDefaultAdmin` _boolean_ | DeleteDefaultAdmin determines whether the default admin user should be deleted after the initial configuration. It is defaulted to true if not provided. |
| `clientUsername` _string_ | ClientUsername is the user to connect to MaxScale. It is defaulted if not provided. |
| `clientPasswordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | ClientPasswordSecretKeyRef is Secret key reference to the password to connect to MaxScale. It is defaulted if not provided. |
| `serverUsername` _string_ | ServerUsername is the user used by MaxScale to connect to MariaDB server. It is defaulted if not provided. |
| `serverPasswordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | ServerPasswordSecretKeyRef is Secret key reference to the password used by MaxScale to connect to MariaDB server. It is defaulted if not provided. |
| `monitorUsername` _string_ | MonitorUsername is the user used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided. |
| `monitorPasswordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | MonitorPasswordSecretKeyRef is Secret key reference to the password used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided. |


#### MaxScaleConfig



MaxScaleConfig defines the MaxScale configuration.

_Appears in:_
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `params` _object (keys:string, values:string)_ | Params is a key value pair of parameters to be used in the MaxScale static configuration file. |
| `volumeClaimTemplate` _[VolumeClaimTemplate](#volumeclaimtemplate)_ | VolumeClaimTemplate provides a template to define the PVCs for storing MaxScale runtime configuration files. |


#### MaxScaleListener



MaxScaleListener defines how the MaxScale server will listen for connections.

_Appears in:_
- [MaxScaleService](#maxscaleservice)

| Field | Description |
| --- | --- |
| `name` _string_ | Name is the identifier of the listener. It is defaulted if not provided |
| `port` _integer_ | Port is the network port where the MaxScale server will listen. |
| `protocol` _string_ | Protocol is the MaxScale protocol to use when communicating with the client. If not provided, it defaults to MariaDBProtocol. |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the listener. |


#### MaxScaleMonitor



MaxScaleMonitor monitors MariaDB server instances

_Appears in:_
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `name` _string_ | Name is the identifier of the monitor. It is defaulted if not provided |
| `module` _[MonitorModule](#monitormodule)_ | Module is the module to use to monitor MariaDB servers. |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Interval used to monitor MariaDB servers. If not provided, it defaults to 2s. |
| `cooperativeMonitoring` _[CooperativeMonitoring](#cooperativemonitoring)_ | CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors. It is defaulted when multiple replicas are configured. |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the monitor. |


#### MaxScaleServer



MaxScaleServer defines a MariaDB server to forward traffic to.

_Appears in:_
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `name` _string_ | Name is the identifier of the MariaDB server. |
| `address` _string_ | Address is the network address of the MariaDB server. |
| `port` _integer_ | Port is the network port of the MariaDB server. If not provided, it defaults to 3306. |
| `protocol` _string_ | Protocol is the MaxScale protocol to use when communicating with this MariaDB server. If not provided, it defaults to MariaDBBackend. |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the server. |


#### MaxScaleService



Services define how the traffic is forwarded to the MariaDB servers.

_Appears in:_
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `name` _string_ | Name is the identifier of the MaxScale service. |
| `router` _[ServiceRouter](#servicerouter)_ | Router is the type of router to use. |
| `listener` _[MaxScaleListener](#maxscalelistener)_ | MaxScaleListener defines how the MaxScale server will listen for connections. |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the monitor. |


#### MaxScaleSpec



MaxScaleSpec defines the desired state of MaxScale

_Appears in:_
- [MaxScale](#maxscale)

| Field | Description |
| --- | --- |
| `command` _string array_ | Command to be used in the Container. |
| `args` _string array_ | Args to be used in the Container. |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |
| `initContainers` _[Container](#container) array_ | InitContainers to be used in the Pod. |
| `sidecarContainers` _[Container](#container) array_ | SidecarContainers to be used in the Pod. |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ | Affinity to be used in the Pod. |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ | Volumes to be used in the Pod. |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints to be used in the Pod. |
| `image` _string_ | Image name to be used by the MaxScale instances. The supported format is `<image>:<tag>`. Only MaxScale official images are supported. |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |
| `servers` _[MaxScaleServer](#maxscaleserver) array_ | Servers are the MariaDB servers to forward traffic to. |
| `services` _[MaxScaleService](#maxscaleservice) array_ | Services define how the traffic is forwarded to the MariaDB servers. |
| `monitor` _[MaxScaleMonitor](#maxscalemonitor)_ | Monitor monitors MariaDB server instances. |
| `admin` _[MaxScaleAdmin](#maxscaleadmin)_ | Admin configures the admin REST API and GUI. |
| `config` _[MaxScaleConfig](#maxscaleconfig)_ | Config defines the MaxScale configuration. |
| `auth` _[MaxScaleAuth](#maxscaleauth)_ | Auth defines the credentials required for MaxScale to connect to MariaDB. |
| `replicas` _integer_ | Replicas indicates the number of desired instances. |
| `podDisruptionBudget` _[PodDisruptionBudget](#poddisruptionbudget)_ | PodDisruptionBudget defines the budget for replica availability. |
| `updateStrategy` _[StatefulSetUpdateStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#statefulsetupdatestrategy-v1-apps)_ | UpdateStrategy defines the update strategy for the StatefulSet object. |
| `kubernetesService` _[ServiceTemplate](#servicetemplate)_ | Service defines templates to configure the Kubernetes Service object. |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconcilizations. If not defined, it defaults to 10s. |


#### Metrics



Metrics defines the metrics for a MariaDB.

_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description |
| --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable Metrics |
| `exporter` _[Exporter](#exporter)_ | Exporter defines the metrics exporter container. |
| `serviceMonitor` _[ServiceMonitor](#servicemonitor)_ | ServiceMonitor defines the ServiceMonior object. |
| `username` _string_ | Username is the username of the monitoring user used by the exporter. |
| `passwordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | PasswordSecretKeyRef is a reference to the password of the monitoring user used by the exporter. |


#### MonitorModule

_Underlying type:_ _string_

MonitorModule defines the type of monitor module

_Appears in:_
- [MaxScaleMonitor](#maxscalemonitor)



#### PodDisruptionBudget



PodDisruptionBudget is the Pod availability bundget for a MariaDB

_Appears in:_
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `minAvailable` _[IntOrString](#intorstring)_ | MinAvailable defines the number of minimum available Pods. |
| `maxUnavailable` _[IntOrString](#intorstring)_ | MaxUnavailable defines the number of maximum unavailable Pods. |


#### PodTemplate



PodTemplate defines a template to configure Container objects.

_Appears in:_
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `initContainers` _[Container](#container) array_ | InitContainers to be used in the Pod. |
| `sidecarContainers` _[Container](#container) array_ | SidecarContainers to be used in the Pod. |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ | Affinity to be used in the Pod. |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ | Volumes to be used in the Pod. |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints to be used in the Pod. |


#### PrimaryGalera



PrimaryGalera is the Galera configuration for the primary node.

_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description |
| --- | --- |
| `podIndex` _integer_ | PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover. |
| `automaticFailover` _boolean_ | AutomaticFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover. |


#### PrimaryReplication



PrimaryReplication is the replication configuration for the primary node.

_Appears in:_
- [Replication](#replication)
- [ReplicationSpec](#replicationspec)

| Field | Description |
| --- | --- |
| `podIndex` _integer_ | PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover. |
| `automaticFailover` _boolean_ | AutomaticFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover. |


#### ReplicaReplication



ReplicaReplication is the replication configuration for the replica nodes.

_Appears in:_
- [Replication](#replication)
- [ReplicationSpec](#replicationspec)

| Field | Description |
| --- | --- |
| `waitPoint` _[WaitPoint](#waitpoint)_ | WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine. More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point. |
| `gtid` _[Gtid](#gtid)_ | Gtid indicates which Global Transaction ID should be used when connecting a replica to the master. See: https://mariadb.com/kb/en/gtid/#using-current_pos-vs-slave_pos. |
| `replPasswordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | ReplPasswordSecretKeyRef provides a reference to the Secret to use as password for the replication user. |
| `connectionTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | ConnectionTimeout to be used when the replica connects to the primary. |
| `connectionRetries` _integer_ | ConnectionRetries to be used when the replica connects to the primary. |
| `syncTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | SyncTimeout defines the timeout for a replica to be synced with the primary when performing a primary switchover. If the timeout is reached, the replica GTID will be reset and the switchover will continue. |


#### Replication



Replication allows you to enable single-master HA via semi-synchronours replication in your MariaDB cluster.

_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description |
| --- | --- |
| `primary` _[PrimaryReplication](#primaryreplication)_ | Primary is the replication configuration for the primary node. |
| `replica` _[ReplicaReplication](#replicareplication)_ | ReplicaReplication is the replication configuration for the replica nodes. |
| `syncBinlog` _boolean_ | SyncBinlog indicates whether the binary log should be synchronized to the disk after every event. It trades off performance for consistency. See: https://mariadb.com/kb/en/replication-and-binary-log-system-variables/#sync_binlog. |
| `enabled` _boolean_ | Enabled is a flag to enable Replication. |


#### ReplicationSpec



ReplicationSpec is the Replication desired state specification.

_Appears in:_
- [Replication](#replication)

| Field | Description |
| --- | --- |
| `primary` _[PrimaryReplication](#primaryreplication)_ | Primary is the replication configuration for the primary node. |
| `replica` _[ReplicaReplication](#replicareplication)_ | ReplicaReplication is the replication configuration for the replica nodes. |
| `syncBinlog` _boolean_ | SyncBinlog indicates whether the binary log should be synchronized to the disk after every event. It trades off performance for consistency. See: https://mariadb.com/kb/en/replication-and-binary-log-system-variables/#sync_binlog. |


#### Restore



Restore is the Schema for the restores API. It is used to define restore jobs and its restoration source.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `mariadb.mmontes.io/v1alpha1`
| `kind` _string_ | `Restore`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[RestoreSpec](#restorespec)_ |  |


#### RestoreSource



RestoreSource defines a source for restoring a MariaDB.

_Appears in:_
- [MariaDBSpec](#mariadbspec)
- [RestoreSpec](#restorespec)

| Field | Description |
| --- | --- |
| `backupRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | BackupRef is a reference to a Backup object. It has priority over S3 and Volume. |
| `s3` _[S3](#s3)_ | S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume. |
| `volume` _[VolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumesource-v1-core)_ | Volume is a Kubernetes Volume object that contains a backup. |
| `targetRecoveryTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective. It is used to determine the closest restoration source in time. |


#### RestoreSpec



RestoreSpec defines the desired state of restore

_Appears in:_
- [Restore](#restore)

| Field | Description |
| --- | --- |
| `backupRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | BackupRef is a reference to a Backup object. It has priority over S3 and Volume. |
| `s3` _[S3](#s3)_ | S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume. |
| `volume` _[VolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumesource-v1-core)_ | Volume is a Kubernetes Volume object that contains a backup. |
| `targetRecoveryTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective. It is used to determine the closest restoration source in time. |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |
| `args` _string array_ | Args to be used in the Restore container. |
| `logLevel` _string_ | LogLevel to be used n the Backup Job. It defaults to 'info'. |
| `backoffLimit` _integer_ | BackoffLimit defines the maximum number of attempts to successfully perform a Backup. |
| `restartPolicy` _[RestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#restartpolicy-v1-core)_ | RestartPolicy to be added to the Backup Job. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ | Affinity to be used in the Restore Pod. |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Restore Pod. |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Restore Pod. |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |


#### S3





_Appears in:_
- [BackupStorage](#backupstorage)
- [RestoreSource](#restoresource)
- [RestoreSpec](#restorespec)

| Field | Description |
| --- | --- |
| `bucket` _string_ | Bucket is the name Name of the bucket to store backups. |
| `endpoint` _string_ | Endpoint is the S3 API endpoint without scheme. |
| `region` _string_ | Region is the S3 region name to use. |
| `accessKeyIdSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | AccessKeyIdSecretKeyRef is a reference to a Secret key containing the S3 access key id. |
| `secretAccessKeySecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | AccessKeyIdSecretKeyRef is a reference to a Secret key containing the S3 secret key. |
| `sessionTokenSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | SessionTokenSecretKeyRef is a reference to a Secret key containing the S3 session token. |
| `tls` _[TLS](#tls)_ | TLS provides the configuration required to establish TLS connections with S3. |
| `prefix` _string_ | Prefix allows backups to be placed under a specific prefix in the bucket. |


#### SQLTemplate



SQLTemplate defines a template to customize SQL objects.

_Appears in:_
- [DatabaseSpec](#databasespec)
- [GrantSpec](#grantspec)
- [UserSpec](#userspec)

| Field | Description |
| --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconcilizations. |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |


#### SST

_Underlying type:_ _string_

SST is the Snapshot State Transfer used when new Pods join the cluster. More info: https://galeracluster.com/library/documentation/sst.html.

_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)



#### Schedule



Schedule contains parameters to define a schedule

_Appears in:_
- [BackupSpec](#backupspec)
- [SqlJobSpec](#sqljobspec)

| Field | Description |
| --- | --- |
| `cron` _string_ | Cron is a cron expression that defines the schedule. |
| `suspend` _boolean_ | Suspend defines whether the schedule is active or not. |


#### SecretTemplate



SecretTemplate defines a template to customize Secret objects.

_Appears in:_
- [ConnectionSpec](#connectionspec)
- [ConnectionTemplate](#connectiontemplate)

| Field | Description |
| --- | --- |
| `labels` _object (keys:string, values:string)_ | Labels to be added to the Secret object. |
| `annotations` _object (keys:string, values:string)_ | Annotations to be added to the Secret object. |
| `key` _string_ | Key to be used in the Secret. |
| `format` _string_ | Format to be used in the Secret. |
| `usernameKey` _string_ | UsernameKey to be used in the Secret. |
| `passwordKey` _string_ | PasswordKey to be used in the Secret. |
| `hostKey` _string_ | HostKey to be used in the Secret. |
| `portKey` _string_ | PortKey to be used in the Secret. |
| `databaseKey` _string_ | DatabaseKey to be used in the Secret. |


#### ServiceMonitor



ServiceMonitor defines a prometheus ServiceMonitor object.

_Appears in:_
- [Metrics](#metrics)

| Field | Description |
| --- | --- |
| `prometheusRelease` _string_ | PrometheusRelease is the release label to add to the ServiceMonitor object. |
| `jobLabel` _string_ | JobLabel to add to the ServiceMonitor object. |
| `interval` _string_ | Interval for scraping metrics. |
| `scrapeTimeout` _string_ | ScrapeTimeout defines the timeout for scraping metrics. |


#### ServiceRouter

_Underlying type:_ _string_

ServiceRouter defines the type of service router.

_Appears in:_
- [MaxScaleService](#maxscaleservice)



#### ServiceTemplate



ServiceTemplate defines a template to customize Service objects.

_Appears in:_
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description |
| --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ | Type is the Service type. One of `ClusterIP`, `NodePort` or `LoadBalancer`. If not defined, it defaults to `ClusterIP`. |
| `labels` _object (keys:string, values:string)_ | Labels to add to the Service metadata. |
| `annotations` _object (keys:string, values:string)_ | Annotations to add to the Service metadata. |
| `loadBalancerIP` _string_ | LoadBalancerIP Service field. |
| `loadBalancerSourceRanges` _string array_ | LoadBalancerSourceRanges Service field. |
| `externalTrafficPolicy` _[ServiceExternalTrafficPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#serviceexternaltrafficpolicy-v1-core)_ | ExternalTrafficPolicy Service field. |
| `sessionAffinity` _[ServiceAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#serviceaffinity-v1-core)_ | SessionAffinity Service field. |
| `allocateLoadBalancerNodePorts` _boolean_ | AllocateLoadBalancerNodePorts Service field. |


#### SqlJob



SqlJob is the Schema for the sqljobs API. It is used to run sql scripts as jobs.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `mariadb.mmontes.io/v1alpha1`
| `kind` _string_ | `SqlJob`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[SqlJobSpec](#sqljobspec)_ |  |


#### SqlJobSpec



SqlJobSpec defines the desired state of SqlJob

_Appears in:_
- [SqlJob](#sqljob)

| Field | Description |
| --- | --- |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |
| `schedule` _[Schedule](#schedule)_ | Schedule defines when the SqlJob will be executed. |
| `username` _string_ | Username to be impersonated when executing the SqlJob. |
| `passwordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | UserPasswordSecretKeyRef is a reference to the impersonated user's password to be used when executing the SqlJob. |
| `database` _string_ | Username to be used when executing the SqlJob. |
| `dependsOn` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | DependsOn defines dependencies with other SqlJob objectecs. |
| `sql` _string_ | Sql is the script to be executed by the SqlJob. |
| `sqlConfigMapKeyRef` _[ConfigMapKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapkeyselector-v1-core)_ | SqlConfigMapKeyRef is a reference to a ConfigMap containing the Sql script. It is defaulted to a ConfigMap with the contents of the Sql field. |
| `backoffLimit` _integer_ | BackoffLimit defines the maximum number of attempts to successfully execute a SqlJob. |
| `restartPolicy` _[RestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#restartpolicy-v1-core)_ | RestartPolicy to be added to the SqlJob Pod. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ | Affinity to be used in the SqlJob Pod. |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the SqlJob Pod. |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the SqlJob Pod. |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |


#### TLS





_Appears in:_
- [S3](#s3)

| Field | Description |
| --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable TLS. |
| `caSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | CASecretKeyRef is a reference to a Secret key containing a CA bundle in PEM format used to establish TLS connections with S3. |


#### User



User is the Schema for the users API.  It is used to define grants as if you were running a 'CREATE USER' statement.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `mariadb.mmontes.io/v1alpha1`
| `kind` _string_ | `User`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[UserSpec](#userspec)_ |  |


#### UserSpec



UserSpec defines the desired state of User

_Appears in:_
- [User](#user)

| Field | Description |
| --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconcilizations. |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |
| `passwordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | PasswordSecretKeyRef is a reference to the password to be used by the User. |
| `maxUserConnections` _integer_ | MaxUserConnections defines the maximum number of connections that the User can have. |
| `name` _string_ | Name overrides the default name provided by metadata.name. |
| `host` _string_ | Host related to the User. |


#### VolumeClaimTemplate



VolumeClaimTemplate defines a template to customize PVC objects.

_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleConfig](#maxscaleconfig)

| Field | Description |
| --- | --- |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeaccessmode-v1-core) array_ | accessModes contains the desired access modes the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1 |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#labelselector-v1-meta)_ | selector is a label query over volumes to consider for binding. |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | resources represents the minimum resources the volume should have. If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements that are lower than previous value but must still be higher than capacity recorded in the status field of the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources |
| `volumeName` _string_ | volumeName is the binding reference to the PersistentVolume backing this claim. |
| `storageClassName` _string_ | storageClassName is the name of the StorageClass required by the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1 |
| `volumeMode` _[PersistentVolumeMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumemode-v1-core)_ | volumeMode defines what type of volume is required by the claim. Value of Filesystem is implied when not included in claim spec. |
| `dataSource` _[TypedLocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#typedlocalobjectreference-v1-core)_ | dataSource field can be used to specify either: * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot) * An existing PVC (PersistentVolumeClaim) If the provisioner or an external controller can support the specified data source, it will create a new volume based on the contents of the specified data source. When the AnyVolumeDataSource feature gate is enabled, dataSource contents will be copied to dataSourceRef, and dataSourceRef contents will be copied to dataSource when dataSourceRef.namespace is not specified. If the namespace is specified, then dataSourceRef will not be copied to dataSource. |
| `dataSourceRef` _[TypedObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#typedobjectreference-v1-core)_ | dataSourceRef specifies the object from which to populate the volume with data, if a non-empty volume is desired. This may be any object from a non-empty API group (non core object) or a PersistentVolumeClaim object. When this field is specified, volume binding will only succeed if the type of the specified object matches some installed volume populator or dynamic provisioner. This field will replace the functionality of the dataSource field and as such if both fields are non-empty, they must have the same value. For backwards compatibility, when namespace isn't specified in dataSourceRef, both fields (dataSource and dataSourceRef) will be set to the same value automatically if one of them is empty and the other is non-empty. When namespace is specified in dataSourceRef, dataSource isn't set to the same value and must be empty. There are three important differences between dataSource and dataSourceRef: * While dataSource only allows two specific types of objects, dataSourceRef allows any non-core object, as well as PersistentVolumeClaim objects. * While dataSource ignores disallowed values (dropping them), dataSourceRef preserves all values, and generates an error if a disallowed value is specified. * While dataSource only allows local objects, dataSourceRef allows objects in any namespaces. (Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled. (Alpha) Using the namespace field of dataSourceRef requires the CrossNamespaceVolumeDataSource feature gate to be enabled. |
| `labels` _object (keys:string, values:string)_ | Labels to be used in the PVC. |
| `annotations` _object (keys:string, values:string)_ | Annotations to be used in the PVC. |


#### WaitPoint

_Underlying type:_ _string_

WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine. More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.

_Appears in:_
- [ReplicaReplication](#replicareplication)



