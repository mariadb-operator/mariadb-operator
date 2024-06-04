# API Reference

## Packages
- [k8s.mariadb.com/v1alpha1](#k8smariadbcomv1alpha1)


## k8s.mariadb.com/v1alpha1

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



#### AffinityConfig



AffinityConfig defines policies to schedule Pods in Nodes.



_Appears in:_
- [BackupSpec](#backupspec)
- [Exporter](#exporter)
- [Job](#job)
- [JobPodTemplate](#jobpodtemplate)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)
- [PodTemplate](#podtemplate)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodeAffinity` _[NodeAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#nodeaffinity-v1-core)_ | Describes node affinity scheduling rules for the pod. |  |  |
| `podAffinity` _[PodAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podaffinity-v1-core)_ | Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)). |  |  |
| `podAntiAffinity` _[PodAntiAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podantiaffinity-v1-core)_ | Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)). |  |  |
| `antiAffinityEnabled` _boolean_ | AntiAffinityEnabled configures PodAntiAffinity so each Pod is scheduled in a different Node, enabling HA.<br />Make sure you have at least as many Nodes available as the replicas to not end up with unscheduled Pods. |  |  |


#### Backup



Backup is the Schema for the backups API. It is used to define backup jobs and its storage.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Backup` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[BackupSpec](#backupspec)_ |  |  |  |


#### BackupSpec



BackupSpec defines the desired state of Backup



_Appears in:_
- [Backup](#backup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: {} <br /> |
| `storage` _[BackupStorage](#backupstorage)_ | Storage to be used in the Backup. |  | Required: {} <br /> |
| `schedule` _[Schedule](#schedule)_ | Schedule defines when the Backup will be taken. |  |  |
| `maxRetention` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | MaxRetention defines the retention policy for backups. Old backups will be cleaned up by the Backup Job.<br />It defaults to 30 days. |  |  |
| `databases` _string array_ | Databases defines the logical databases to be backed up. If not provided, all databases are backed up. |  |  |
| `ignoreGlobalPriv` _boolean_ | IgnoreGlobalPriv indicates to ignore the mysql.global_priv in backups.<br />If not provided, it will default to true when the referred MariaDB instance has Galera enabled and otherwise to false.<br />See: https://github.com/mariadb-operator/mariadb-operator/issues/556 |  |  |
| `logLevel` _string_ | LogLevel to be used n the Backup Job. It defaults to 'info'. | info |  |
| `backoffLimit` _integer_ | BackoffLimit defines the maximum number of attempts to successfully take a Backup. |  |  |
| `restartPolicy` _[RestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#restartpolicy-v1-core)_ | RestartPolicy to be added to the Backup Pod. | OnFailure | Enum: [Always OnFailure Never] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |


#### BackupStorage



BackupStorage defines the storage for a Backup.



_Appears in:_
- [BackupSpec](#backupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `s3` _[S3](#s3)_ | S3 defines the configuration to store backups in a S3 compatible storage. |  |  |
| `persistentVolumeClaim` _[PersistentVolumeClaimSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimspec-v1-core)_ | PersistentVolumeClaim is a Kubernetes PVC specification. |  |  |
| `volume` _[VolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumesource-v1-core)_ | Volume is a Kubernetes volume specification. |  |  |


#### BootstrapFrom



BootstrapFrom defines a source to bootstrap MariaDB from.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backupRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | BackupRef is a reference to a Backup object. It has priority over S3 and Volume. |  |  |
| `s3` _[S3](#s3)_ | S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume. |  |  |
| `volume` _[VolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumesource-v1-core)_ | Volume is a Kubernetes Volume object that contains a backup. |  |  |
| `targetRecoveryTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.<br />It is used to determine the closest restoration source in time. |  |  |
| `restoreJob` _[Job](#job)_ | RestoreJob defines additional properties for the Job used to perform the Restore. |  |  |


#### Connection



Connection is the Schema for the connections API. It is used to configure connection strings for the applications connecting to MariaDB.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Connection` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ConnectionSpec](#connectionspec)_ |  |  |  |


#### ConnectionSpec



ConnectionSpec defines the desired state of Connection



_Appears in:_
- [Connection](#connection)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretName` _string_ | SecretName to be used in the Connection. |  |  |
| `secretTemplate` _[SecretTemplate](#secrettemplate)_ | SecretTemplate to be used in the Connection. |  |  |
| `healthCheck` _[HealthCheck](#healthcheck)_ | HealthCheck to be used in the Connection. |  |  |
| `params` _object (keys:string, values:string)_ | Params to be used in the Connection. |  |  |
| `serviceName` _string_ | ServiceName to be used in the Connection. |  |  |
| `port` _integer_ | Port to connect to. If not provided, it defaults to the MariaDB port or to the first MaxScale listener. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to the MariaDB to connect to. Either MariaDBRef or MaxScaleRef must be provided. |  |  |
| `maxScaleRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectreference-v1-core)_ | MaxScaleRef is a reference to the MaxScale to connect to. Either MariaDBRef or MaxScaleRef must be provided. |  |  |
| `username` _string_ | Username to use for configuring the Connection. |  | Required: {} <br /> |
| `passwordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | PasswordSecretKeyRef is a reference to the password to use for configuring the Connection.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  | Required: {} <br /> |
| `host` _string_ | Host to connect to. If not provided, it defaults to the MariaDB host or to the MaxScale host. |  |  |
| `database` _string_ | Database to use when configuring the Connection. |  |  |


#### ConnectionTemplate



ConnectionTemplate defines a template to customize Connection objects.



_Appears in:_
- [ConnectionSpec](#connectionspec)
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretName` _string_ | SecretName to be used in the Connection. |  |  |
| `secretTemplate` _[SecretTemplate](#secrettemplate)_ | SecretTemplate to be used in the Connection. |  |  |
| `healthCheck` _[HealthCheck](#healthcheck)_ | HealthCheck to be used in the Connection. |  |  |
| `params` _object (keys:string, values:string)_ | Params to be used in the Connection. |  |  |
| `serviceName` _string_ | ServiceName to be used in the Connection. |  |  |
| `port` _integer_ | Port to connect to. If not provided, it defaults to the MariaDB port or to the first MaxScale listener. |  |  |


#### Container



Container object definition.



_Appears in:_
- [Exporter](#exporter)
- [Galera](#galera)
- [GaleraSpec](#galeraspec)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)
- [PodTemplate](#podtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `image` _string_ | Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`. |  | Required: {} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |


#### ContainerTemplate



ContainerTemplate defines a template to configure Container objects.



_Appears in:_
- [Container](#container)
- [Exporter](#exporter)
- [GaleraAgent](#galeraagent)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |


#### CooperativeMonitoring

_Underlying type:_ _string_

CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors.
See: https://mariadb.com/docs/server/architecture/components/maxscale/monitors/mariadbmon/use-cooperative-locking-ha-maxscale-mariadb-monitor/



_Appears in:_
- [MaxScaleMonitor](#maxscalemonitor)



#### Database



Database is the Schema for the databases API. It is used to define a logical database as if you were running a 'CREATE DATABASE' statement.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Database` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[DatabaseSpec](#databasespec)_ |  |  |  |


#### DatabaseSpec



DatabaseSpec defines the desired state of Database



_Appears in:_
- [Database](#database)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: {} <br /> |
| `characterSet` _string_ | CharacterSet to use in the Database. | utf8 |  |
| `collate` _string_ | Collate to use in the Database. | utf8_general_ci |  |
| `name` _string_ | Name overrides the default Database name provided by metadata.name. |  | MaxLength: 80 <br /> |


#### Exporter



Exporter defines a metrics exporter container.



_Appears in:_
- [MariadbMetrics](#mariadbmetrics)
- [MaxScaleMetrics](#maxscalemetrics)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `initContainers` _[Container](#container) array_ | InitContainers to be used in the Pod. |  |  |
| `sidecarContainers` _[Container](#container) array_ | SidecarContainers to be used in the Pod. |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ | Volumes to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints to be used in the Pod. |  |  |
| `image` _string_ | Image name to be used as metrics exporter. The supported format is `<image>:<tag>`.<br />Only mysqld-exporter >= v0.15.0 is supported: https://github.com/prometheus/mysqld_exporter |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `port` _integer_ | Port where the exporter will be listening for connections. |  |  |


#### Galera



Galera allows you to enable multi-master HA via Galera in your MariaDB cluster.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `primary` _[PrimaryGalera](#primarygalera)_ | Primary is the Galera configuration for the primary node. |  |  |
| `sst` _[SST](#sst)_ | SST is the Snapshot State Transfer used when new Pods join the cluster.<br />More info: https://galeracluster.com/library/documentation/sst.html. |  | Enum: [rsync mariabackup mysqldump] <br /> |
| `availableWhenDonor` _boolean_ | AvailableWhenDonor indicates whether a donor node should be responding to queries. It defaults to false. |  |  |
| `galeraLibPath` _string_ | GaleraLibPath is a path inside the MariaDB image to the wsrep provider plugin. It is defaulted if not provided.<br />More info: https://galeracluster.com/library/documentation/mysql-wsrep-options.html#wsrep-provider. |  |  |
| `replicaThreads` _integer_ | ReplicaThreads is the number of replica threads used to apply Galera write sets in parallel.<br />More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_slave_threads. |  |  |
| `providerOptions` _object (keys:string, values:string)_ | ProviderOptions is map of Galera configuration parameters.<br />More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_provider_options. |  |  |
| `agent` _[GaleraAgent](#galeraagent)_ | GaleraAgent is a sidecar agent that co-operates with mariadb-operator. |  |  |
| `recovery` _[GaleraRecovery](#galerarecovery)_ | GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.<br />More info: https://galeracluster.com/library/documentation/crash-recovery.html. |  |  |
| `initContainer` _[Container](#container)_ | InitContainer is an init container that co-operates with mariadb-operator. |  |  |
| `initJob` _[Job](#job)_ | InitJob defines additional properties for the Job used to perform the initialization. |  |  |
| `config` _[GaleraConfig](#galeraconfig)_ | GaleraConfig defines storage options for the Galera configuration files. |  |  |
| `enabled` _boolean_ | Enabled is a flag to enable Galera. |  |  |


#### GaleraAgent



GaleraAgent is a sidecar agent that co-operates with mariadb-operator.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `image` _string_ | Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `port` _integer_ | Port where the agent will be listening for connections. |  |  |
| `kubernetesAuth` _[KubernetesAuth](#kubernetesauth)_ | KubernetesAuth to be used by the agent container |  |  |
| `gracefulShutdownTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | GracefulShutdownTimeout is the time we give to the agent container in order to gracefully terminate in-flight requests. |  |  |


#### GaleraConfig



GaleraConfig defines storage options for the Galera configuration files.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `reuseStorageVolume` _boolean_ | ReuseStorageVolume indicates that storage volume used by MariaDB should be reused to store the Galera configuration files.<br />It defaults to false, which implies that a dedicated volume for the Galera configuration files is provisioned. |  |  |
| `volumeClaimTemplate` _[VolumeClaimTemplate](#volumeclaimtemplate)_ | VolumeClaimTemplate is a template for the PVC that will contain the Galera configuration files shared between the InitContainer, Agent and MariaDB. |  |  |


#### GaleraRecovery



GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
More info: https://galeracluster.com/library/documentation/crash-recovery.html.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable GaleraRecovery. |  |  |
| `minClusterSize` _[IntOrString](#intorstring)_ | MinClusterSize is the minimum number of replicas to consider the cluster healthy. It can be either a number of replicas (3) or a percentage (50%).<br />If Galera consistently reports less replicas than this value for the given 'ClusterHealthyTimeout' interval, a cluster recovery is iniated.<br />It defaults to '50%' of the replicas specified by the MariaDB object. |  |  |
| `clusterMonitorInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | ClusterMonitorInterval represents the interval used to monitor the Galera cluster health. |  |  |
| `clusterHealthyTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | ClusterHealthyTimeout represents the duration at which a Galera cluster, that consistently failed health checks,<br />is considered unhealthy, and consequently the Galera recovery process will be initiated by the operator. |  |  |
| `clusterBootstrapTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | ClusterBootstrapTimeout is the time limit for bootstrapping a cluster.<br />Once this timeout is reached, the Galera recovery state is reset and a new cluster bootstrap will be attempted. |  |  |
| `podRecoveryTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | PodRecoveryTimeout is the time limit for recevorying the sequence of a Pod during the cluster recovery.<br />Once this timeout is reached, the Pod is restarted. |  |  |
| `podSyncTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | PodSyncTimeout is the time limit for a Pod to join the cluster after having performed a cluster bootstrap during the cluster recovery.<br />Once this timeout is reached, the Pod is restarted. |  |  |


#### GaleraSpec



GaleraSpec is the Galera desired state specification.



_Appears in:_
- [Galera](#galera)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `primary` _[PrimaryGalera](#primarygalera)_ | Primary is the Galera configuration for the primary node. |  |  |
| `sst` _[SST](#sst)_ | SST is the Snapshot State Transfer used when new Pods join the cluster.<br />More info: https://galeracluster.com/library/documentation/sst.html. |  | Enum: [rsync mariabackup mysqldump] <br /> |
| `availableWhenDonor` _boolean_ | AvailableWhenDonor indicates whether a donor node should be responding to queries. It defaults to false. |  |  |
| `galeraLibPath` _string_ | GaleraLibPath is a path inside the MariaDB image to the wsrep provider plugin. It is defaulted if not provided.<br />More info: https://galeracluster.com/library/documentation/mysql-wsrep-options.html#wsrep-provider. |  |  |
| `replicaThreads` _integer_ | ReplicaThreads is the number of replica threads used to apply Galera write sets in parallel.<br />More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_slave_threads. |  |  |
| `providerOptions` _object (keys:string, values:string)_ | ProviderOptions is map of Galera configuration parameters.<br />More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_provider_options. |  |  |
| `agent` _[GaleraAgent](#galeraagent)_ | GaleraAgent is a sidecar agent that co-operates with mariadb-operator. |  |  |
| `recovery` _[GaleraRecovery](#galerarecovery)_ | GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.<br />More info: https://galeracluster.com/library/documentation/crash-recovery.html. |  |  |
| `initContainer` _[Container](#container)_ | InitContainer is an init container that co-operates with mariadb-operator. |  |  |
| `initJob` _[Job](#job)_ | InitJob defines additional properties for the Job used to perform the initialization. |  |  |
| `config` _[GaleraConfig](#galeraconfig)_ | GaleraConfig defines storage options for the Galera configuration files. |  |  |


#### GeneratedSecretKeyRef



GeneratedSecretKeyRef defines a reference to a Secret that can be automatically generated by mariadb-operator if needed.



_Appears in:_
- [MariaDBSpec](#mariadbspec)
- [MariadbMetrics](#mariadbmetrics)
- [MaxScaleAuth](#maxscaleauth)
- [ReplicaReplication](#replicareplication)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names<br />TODO: Add other useful fields. apiVersion, kind, uid? |  |  |
| `key` _string_ | The key of the secret to select from.  Must be a valid secret key. |  |  |
| `optional` _boolean_ | Specify whether the Secret or its key must be defined |  |  |
| `generate` _boolean_ | Generate indicates whether the Secret should be generated if the Secret referenced is not present. | false |  |


#### Grant



Grant is the Schema for the grants API. It is used to define grants as if you were running a 'GRANT' statement.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Grant` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[GrantSpec](#grantspec)_ |  |  |  |


#### GrantSpec



GrantSpec defines the desired state of Grant



_Appears in:_
- [Grant](#grant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: {} <br /> |
| `privileges` _string array_ | Privileges to use in the Grant. |  | MinItems: 1 <br />Required: {} <br /> |
| `database` _string_ | Database to use in the Grant. | * |  |
| `table` _string_ | Table to use in the Grant. | * |  |
| `username` _string_ | Username to use in the Grant. |  | Required: {} <br /> |
| `host` _string_ | Host to use in the Grant. It can be localhost, an IP or '%'. |  |  |
| `grantOption` _boolean_ | GrantOption to use in the Grant. | false |  |


#### Gtid

_Underlying type:_ _string_

Gtid indicates which Global Transaction ID should be used when connecting a replica to the master.
See: https://mariadb.com/kb/en/gtid/#using-current_pos-vs-slave_pos.



_Appears in:_
- [ReplicaReplication](#replicareplication)



#### HealthCheck



HealthCheck defines intervals for performing health checks.



_Appears in:_
- [ConnectionSpec](#connectionspec)
- [ConnectionTemplate](#connectiontemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Interval used to perform health checks. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the interval used to perform health check retries. |  |  |


#### Job



Job defines a Job used to be used with MariaDB.



_Appears in:_
- [BootstrapFrom](#bootstrapfrom)
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |


#### JobContainerTemplate



JobContainerTemplate defines a template to configure Container objects that run in a Job.



_Appears in:_
- [BackupSpec](#backupspec)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |


#### JobPodTemplate



JobPodTemplate defines a template to configure Container objects that run in a Job.



_Appears in:_
- [BackupSpec](#backupspec)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |


#### KubernetesAuth



KubernetesAuth refers to the Kubernetes authentication mechanism utilized for establishing a connection from the operator to the agent.
The agent validates the legitimacy of the service account token provided as an Authorization header by creating a TokenReview resource.



_Appears in:_
- [GaleraAgent](#galeraagent)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable KubernetesAuth |  |  |
| `authDelegatorRoleName` _string_ | AuthDelegatorRoleName is the name of the ClusterRoleBinding that is associated with the "system:auth-delegator" ClusterRole.<br />It is necessary for creating TokenReview objects in order for the agent to validate the service account token. |  |  |


#### MariaDB



MariaDB is the Schema for the mariadbs API. It is used to define MariaDB clusters.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `MariaDB` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[MariaDBSpec](#mariadbspec)_ |  |  |  |


#### MariaDBMaxScaleSpec



MariaDBMaxScaleSpec defines a reduced version of MaxScale to be used with the current MariaDB.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable a MaxScale instance to be used with the current MariaDB. |  |  |
| `image` _string_ | Image name to be used by the MaxScale instances. The supported format is `<image>:<tag>`.<br />Only MariaDB official images are supported. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `services` _[MaxScaleService](#maxscaleservice) array_ | Services define how the traffic is forwarded to the MariaDB servers. |  |  |
| `monitor` _[MaxScaleMonitor](#maxscalemonitor)_ | Monitor monitors MariaDB server instances. |  |  |
| `admin` _[MaxScaleAdmin](#maxscaleadmin)_ | Admin configures the admin REST API and GUI. |  |  |
| `config` _[MaxScaleConfig](#maxscaleconfig)_ | Config defines the MaxScale configuration. |  |  |
| `auth` _[MaxScaleAuth](#maxscaleauth)_ | Auth defines the credentials required for MaxScale to connect to MariaDB. |  |  |
| `metrics` _[MaxScaleMetrics](#maxscalemetrics)_ | Metrics configures metrics and how to scrape them. |  |  |
| `connection` _[ConnectionTemplate](#connectiontemplate)_ | Connection provides a template to define the Connection for MaxScale. |  |  |
| `replicas` _integer_ | Replicas indicates the number of desired instances. |  |  |
| `podDisruptionBudget` _[PodDisruptionBudget](#poddisruptionbudget)_ | PodDisruptionBudget defines the budget for replica availability. |  |  |
| `updateStrategy` _[StatefulSetUpdateStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#statefulsetupdatestrategy-v1-apps)_ | UpdateStrategy defines the update strategy for the StatefulSet object. |  |  |
| `kubernetesService` _[ServiceTemplate](#servicetemplate)_ | KubernetesService defines a template for a Kubernetes Service object to connect to MaxScale. |  |  |
| `guiKubernetesService` _[ServiceTemplate](#servicetemplate)_ | GuiKubernetesService define a template for a Kubernetes Service object to connect to MaxScale's GUI. |  |  |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |


#### MariaDBRef



MariaDBRef is a reference to a MariaDB object.



_Appears in:_
- [BackupSpec](#backupspec)
- [ConnectionSpec](#connectionspec)
- [DatabaseSpec](#databasespec)
- [GrantSpec](#grantspec)
- [MaxScaleSpec](#maxscalespec)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)
- [UserSpec](#userspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind of the referent.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `namespace` _string_ | Namespace of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/ |  |  |
| `name` _string_ | Name of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  |  |
| `uid` _[UID](#uid)_ | UID of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids |  |  |
| `apiVersion` _string_ | API version of the referent. |  |  |
| `resourceVersion` _string_ | Specific resourceVersion to which this reference is made, if any.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency |  |  |
| `fieldPath` _string_ | If referring to a piece of an object instead of an entire object, this string<br />should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2].<br />For example, if the object reference is to a container within a pod, this would take on a value like:<br />"spec.containers{name}" (where "name" refers to the name of the container that triggered<br />the event) or if no container name is specified "spec.containers[2]" (container with<br />index 2 in this pod). This syntax is chosen only to have some well-defined way of<br />referencing a part of an object.<br />TODO: this design is not final and this field is subject to change in the future. |  |  |
| `waitForIt` _boolean_ | WaitForIt indicates whether the controller using this reference should wait for MariaDB to be ready. | true |  |


#### MariaDBSpec



MariaDBSpec defines the desired state of MariaDB



_Appears in:_
- [MariaDB](#mariadb)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `initContainers` _[Container](#container) array_ | InitContainers to be used in the Pod. |  |  |
| `sidecarContainers` _[Container](#container) array_ | SidecarContainers to be used in the Pod. |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ | Volumes to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints to be used in the Pod. |  |  |
| `image` _string_ | Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`.<br />Only MariaDB official images are supported. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |
| `rootPasswordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | RootPasswordSecretKeyRef is a reference to a Secret key containing the root password. |  |  |
| `rootEmptyPassword` _boolean_ | RootEmptyPassword indicates if the root password should be empty. Don't use this feature in production, it is only intended for development and test environments. |  |  |
| `database` _string_ | Database is the initial database to be created by the operator once MariaDB is ready. |  |  |
| `username` _string_ | Username is the initial username to be created by the operator once MariaDB is ready. It has all privileges on the initial database. |  |  |
| `passwordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | PasswordSecretKeyRef is a reference to a Secret that contains the password for the initial user.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `myCnf` _string_ | MyCnf allows to specify the my.cnf file mounted by Mariadb.<br />Updating this field will trigger an update to the Mariadb resource. |  |  |
| `myCnfConfigMapKeyRef` _[ConfigMapKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapkeyselector-v1-core)_ | MyCnfConfigMapKeyRef is a reference to the my.cnf config file provided via a ConfigMap.<br />If not provided, it will be defaulted with a reference to a ConfigMap containing the MyCnf field.<br />If the referred ConfigMap is labeled with "k8s.mariadb.com/watch", an update to the Mariadb resource will be triggered when the ConfigMap is updated. |  |  |
| `bootstrapFrom` _[BootstrapFrom](#bootstrapfrom)_ | BootstrapFrom defines a source to bootstrap from. |  |  |
| `storage` _[Storage](#storage)_ | Storage defines the storage options to be used for provisioning the PVCs mounted by MariaDB. |  |  |
| `metrics` _[MariadbMetrics](#mariadbmetrics)_ | Metrics configures metrics and how to scrape them. |  |  |
| `replication` _[Replication](#replication)_ | Replication configures high availability via replication. This feature is still in alpha, use Galera if you are looking for a more production-ready HA. |  |  |
| `galera` _[Galera](#galera)_ | Replication configures high availability via Galera. |  |  |
| `maxScaleRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectreference-v1-core)_ | MaxScaleRef is a reference to a MaxScale resource to be used with the current MariaDB.<br />Providing this field implies delegating high availability tasks such as primary failover to MaxScale. |  |  |
| `maxScale` _[MariaDBMaxScaleSpec](#mariadbmaxscalespec)_ | MaxScale is the MaxScale specification that defines the MaxScale resource to be used with the current MariaDB.<br />When enabling this field, MaxScaleRef is automatically set. |  |  |
| `replicas` _integer_ | Replicas indicates the number of desired instances. | 1 |  |
| `port` _integer_ | Port where the instances will be listening for connections. | 3306 |  |
| `podDisruptionBudget` _[PodDisruptionBudget](#poddisruptionbudget)_ | PodDisruptionBudget defines the budget for replica availability. |  |  |
| `updateStrategy` _[UpdateStrategy](#updatestrategy)_ | UpdateStrategy defines how a MariaDB resource is updated. |  |  |
| `service` _[ServiceTemplate](#servicetemplate)_ | Service defines templates to configure the general Service object. |  |  |
| `connection` _[ConnectionTemplate](#connectiontemplate)_ | Connection defines templates to configure the general Connection object. |  |  |
| `primaryService` _[ServiceTemplate](#servicetemplate)_ | PrimaryService defines templates to configure the primary Service object. |  |  |
| `primaryConnection` _[ConnectionTemplate](#connectiontemplate)_ | PrimaryConnection defines templates to configure the primary Connection object. |  |  |
| `secondaryService` _[ServiceTemplate](#servicetemplate)_ | SecondaryService defines templates to configure the secondary Service object. |  |  |
| `secondaryConnection` _[ConnectionTemplate](#connectiontemplate)_ | SecondaryConnection defines templates to configure the secondary Connection object. |  |  |


#### MariadbMetrics



MariadbMetrics defines the metrics for a MariaDB.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable Metrics |  |  |
| `exporter` _[Exporter](#exporter)_ | Exporter defines the metrics exporter container. |  |  |
| `serviceMonitor` _[ServiceMonitor](#servicemonitor)_ | ServiceMonitor defines the ServiceMonior object. |  |  |
| `username` _string_ | Username is the username of the monitoring user used by the exporter. |  |  |
| `passwordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | PasswordSecretKeyRef is a reference to the password of the monitoring user used by the exporter.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |


#### MaxScale



MaxScale is the Schema for the maxscales API. It is used to define MaxScale clusters.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `MaxScale` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[MaxScaleSpec](#maxscalespec)_ |  |  |  |


#### MaxScaleAdmin



MaxScaleAdmin configures the admin REST API and GUI.



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `port` _integer_ | Port where the admin REST API and GUI will be exposed. |  |  |
| `guiEnabled` _boolean_ | GuiEnabled indicates whether the admin GUI should be enabled. |  |  |


#### MaxScaleAuth



MaxScaleAuth defines the credentials required for MaxScale to connect to MariaDB.



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `generate` _boolean_ | Generate  defies whether the operator should generate users and grants for MaxScale to work.<br />It only supports MariaDBs specified via spec.mariaDbRef. |  |  |
| `adminUsername` _string_ | AdminUsername is an admin username to call the admin REST API. It is defaulted if not provided. |  |  |
| `adminPasswordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | AdminPasswordSecretKeyRef is Secret key reference to the admin password to call the admin REST API. It is defaulted if not provided. |  |  |
| `deleteDefaultAdmin` _boolean_ | DeleteDefaultAdmin determines whether the default admin user should be deleted after the initial configuration. If not provided, it defaults to true. |  |  |
| `metricsUsername` _string_ | MetricsUsername is an metrics username to call the REST API. It is defaulted if metrics are enabled. |  |  |
| `metricsPasswordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | MetricsPasswordSecretKeyRef is Secret key reference to the metrics password to call the admib REST API. It is defaulted if metrics are enabled.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `clientUsername` _string_ | ClientUsername is the user to connect to MaxScale. It is defaulted if not provided. |  |  |
| `clientPasswordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | ClientPasswordSecretKeyRef is Secret key reference to the password to connect to MaxScale. It is defaulted if not provided.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `clientMaxConnections` _integer_ | ClientMaxConnections defines the maximum number of connections that the client can establish.<br />If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.<br />It defaults to 30 times the number of MaxScale replicas. |  |  |
| `serverUsername` _string_ | ServerUsername is the user used by MaxScale to connect to MariaDB server. It is defaulted if not provided. |  |  |
| `serverPasswordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | ServerPasswordSecretKeyRef is Secret key reference to the password used by MaxScale to connect to MariaDB server. It is defaulted if not provided.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `serverMaxConnections` _integer_ | ServerMaxConnections defines the maximum number of connections that the server can establish.<br />If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.<br />It defaults to 30 times the number of MaxScale replicas. |  |  |
| `monitorUsername` _string_ | MonitorUsername is the user used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided. |  |  |
| `monitorPasswordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | MonitorPasswordSecretKeyRef is Secret key reference to the password used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `monitorMaxConnections` _integer_ | MonitorMaxConnections defines the maximum number of connections that the monitor can establish.<br />If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.<br />It defaults to 30 times the number of MaxScale replicas. |  |  |
| `syncUsername` _string_ | MonitoSyncUsernamerUsername is the user used by MaxScale config sync to connect to MariaDB server. It is defaulted when HA is enabled. |  |  |
| `syncPasswordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | SyncPasswordSecretKeyRef is Secret key reference to the password used by MaxScale config to connect to MariaDB server. It is defaulted when HA is enabled.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `syncMaxConnections` _integer_ | SyncMaxConnections defines the maximum number of connections that the sync can establish.<br />If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.<br />It defaults to 30 times the number of MaxScale replicas. |  |  |


#### MaxScaleConfig



MaxScaleConfig defines the MaxScale configuration.



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `params` _object (keys:string, values:string)_ | Params is a key value pair of parameters to be used in the MaxScale static configuration file.<br />Any parameter supported by MaxScale may be specified here. See reference:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#global-settings. |  |  |
| `volumeClaimTemplate` _[VolumeClaimTemplate](#volumeclaimtemplate)_ | VolumeClaimTemplate provides a template to define the PVCs for storing MaxScale runtime configuration files. It is defaulted if not provided. |  |  |
| `sync` _[MaxScaleConfigSync](#maxscaleconfigsync)_ | Sync defines how to replicate configuration across MaxScale replicas. It is defaulted when HA is enabled. |  |  |


#### MaxScaleConfigSync



MaxScaleConfigSync defines how the config changes are replicated across replicas.



_Appears in:_
- [MaxScaleConfig](#maxscaleconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `database` _string_ | Database is the MariaDB logical database where the 'maxscale_config' table will be created in order to persist and synchronize config changes. If not provided, it defaults to 'mysql'. |  |  |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Interval defines the config synchronization interval. It is defaulted if not provided. |  |  |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Interval defines the config synchronization timeout. It is defaulted if not provided. |  |  |


#### MaxScaleListener



MaxScaleListener defines how the MaxScale server will listen for connections.



_Appears in:_
- [MaxScaleService](#maxscaleservice)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not. Feature flag --feature-maxscale-suspend is required in the controller to enable this. |  |  |
| `name` _string_ | Name is the identifier of the listener. It is defaulted if not provided |  |  |
| `port` _integer_ | Port is the network port where the MaxScale server will listen. |  | Required: {} <br /> |
| `protocol` _string_ | Protocol is the MaxScale protocol to use when communicating with the client. If not provided, it defaults to MariaDBProtocol. |  |  |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the listener.<br />Any parameter supported by MaxScale may be specified here. See reference:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#listener_1. |  |  |


#### MaxScaleMetrics



MaxScaleMetrics defines the metrics for a Maxscale.



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable Metrics |  |  |
| `exporter` _[Exporter](#exporter)_ | Exporter defines the metrics exporter container. |  |  |
| `serviceMonitor` _[ServiceMonitor](#servicemonitor)_ | ServiceMonitor defines the ServiceMonior object. |  |  |


#### MaxScaleMonitor



MaxScaleMonitor monitors MariaDB server instances



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not. Feature flag --feature-maxscale-suspend is required in the controller to enable this. |  |  |
| `name` _string_ | Name is the identifier of the monitor. It is defaulted if not provided. |  |  |
| `module` _[MonitorModule](#monitormodule)_ | Module is the module to use to monitor MariaDB servers. It is mandatory when no MariaDB reference is provided. |  |  |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Interval used to monitor MariaDB servers. It is defaulted if not provided. |  |  |
| `cooperativeMonitoring` _[CooperativeMonitoring](#cooperativemonitoring)_ | CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors. It is defaulted when HA is enabled. |  | Enum: [majority_of_all majority_of_running] <br /> |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the monitor.<br />Any parameter supported by MaxScale may be specified here. See reference:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-common-monitor-parameters/.<br />Monitor specific parameter are also suported:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-galera-monitor/#galera-monitor-optional-parameters.<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-monitor/#configuration. |  |  |


#### MaxScaleServer



MaxScaleServer defines a MariaDB server to forward traffic to.



_Appears in:_
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the identifier of the MariaDB server. |  | Required: {} <br /> |
| `address` _string_ | Address is the network address of the MariaDB server. |  | Required: {} <br /> |
| `port` _integer_ | Port is the network port of the MariaDB server. If not provided, it defaults to 3306. |  |  |
| `protocol` _string_ | Protocol is the MaxScale protocol to use when communicating with this MariaDB server. If not provided, it defaults to MariaDBBackend. |  |  |
| `maintenance` _boolean_ | Maintenance indicates whether the server is in maintenance mode. |  |  |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the server.<br />Any parameter supported by MaxScale may be specified here. See reference:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#server_1. |  |  |


#### MaxScaleService



Services define how the traffic is forwarded to the MariaDB servers.



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not. Feature flag --feature-maxscale-suspend is required in the controller to enable this. |  |  |
| `name` _string_ | Name is the identifier of the MaxScale service. |  | Required: {} <br /> |
| `router` _[ServiceRouter](#servicerouter)_ | Router is the type of router to use. |  | Enum: [readwritesplit readconnroute] <br />Required: {} <br /> |
| `listener` _[MaxScaleListener](#maxscalelistener)_ | MaxScaleListener defines how the MaxScale server will listen for connections. |  | Required: {} <br /> |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the service.<br />Any parameter supported by MaxScale may be specified here. See reference:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#service_1.<br />Router specific parameter are also suported:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-readwritesplit/#configuration.<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-readconnroute/#configuration. |  |  |


#### MaxScaleSpec



MaxScaleSpec defines the desired state of MaxScale.



_Appears in:_
- [MaxScale](#maxscale)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envfromsource-v1-core) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#probe-v1-core)_ | ReadinessProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `initContainers` _[Container](#container) array_ | InitContainers to be used in the Pod. |  |  |
| `sidecarContainers` _[Container](#container) array_ | SidecarContainers to be used in the Pod. |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ | Volumes to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints to be used in the Pod. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to the MariaDB that MaxScale points to. It is used to initialize the servers field. |  |  |
| `servers` _[MaxScaleServer](#maxscaleserver) array_ | Servers are the MariaDB servers to forward traffic to. It is required if 'spec.mariaDbRef' is not provided. |  |  |
| `image` _string_ | Image name to be used by the MaxScale instances. The supported format is `<image>:<tag>`.<br />Only MaxScale official images are supported. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |
| `services` _[MaxScaleService](#maxscaleservice) array_ | Services define how the traffic is forwarded to the MariaDB servers. It is defaulted if not provided. |  |  |
| `monitor` _[MaxScaleMonitor](#maxscalemonitor)_ | Monitor monitors MariaDB server instances. It is required if 'spec.mariaDbRef' is not provided. |  |  |
| `admin` _[MaxScaleAdmin](#maxscaleadmin)_ | Admin configures the admin REST API and GUI. |  |  |
| `config` _[MaxScaleConfig](#maxscaleconfig)_ | Config defines the MaxScale configuration. |  |  |
| `auth` _[MaxScaleAuth](#maxscaleauth)_ | Auth defines the credentials required for MaxScale to connect to MariaDB. |  |  |
| `metrics` _[MaxScaleMetrics](#maxscalemetrics)_ | Metrics configures metrics and how to scrape them. |  |  |
| `connection` _[ConnectionTemplate](#connectiontemplate)_ | Connection provides a template to define the Connection for MaxScale. |  |  |
| `replicas` _integer_ | Replicas indicates the number of desired instances. | 1 |  |
| `podDisruptionBudget` _[PodDisruptionBudget](#poddisruptionbudget)_ | PodDisruptionBudget defines the budget for replica availability. |  |  |
| `updateStrategy` _[StatefulSetUpdateStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#statefulsetupdatestrategy-v1-apps)_ | UpdateStrategy defines the update strategy for the StatefulSet object. |  |  |
| `kubernetesService` _[ServiceTemplate](#servicetemplate)_ | KubernetesService defines a template for a Kubernetes Service object to connect to MaxScale. |  |  |
| `guiKubernetesService` _[ServiceTemplate](#servicetemplate)_ | GuiKubernetesService defines a template for a Kubernetes Service object to connect to MaxScale's GUI. |  |  |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. If not defined, it defaults to 10s. |  |  |


#### Metadata



Metadata defines the metadata to added to resources.



_Appears in:_
- [BackupSpec](#backupspec)
- [Exporter](#exporter)
- [Job](#job)
- [JobPodTemplate](#jobpodtemplate)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)
- [PodTemplate](#podtemplate)
- [RestoreSpec](#restorespec)
- [SecretTemplate](#secrettemplate)
- [ServiceTemplate](#servicetemplate)
- [SqlJobSpec](#sqljobspec)
- [VolumeClaimTemplate](#volumeclaimtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labels` _object (keys:string, values:string)_ | Labels to be added to children resources. |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations to be added to children resources. |  |  |


#### MonitorModule

_Underlying type:_ _string_

MonitorModule defines the type of monitor module



_Appears in:_
- [MaxScaleMonitor](#maxscalemonitor)



#### PodDisruptionBudget



PodDisruptionBudget is the Pod availability bundget for a MariaDB



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minAvailable` _[IntOrString](#intorstring)_ | MinAvailable defines the number of minimum available Pods. |  | Required: {} <br /> |
| `maxUnavailable` _[IntOrString](#intorstring)_ | MaxUnavailable defines the number of maximum unavailable Pods. |  | Required: {} <br /> |


#### PodTemplate



PodTemplate defines a template to configure Container objects.



_Appears in:_
- [Exporter](#exporter)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `initContainers` _[Container](#container) array_ | InitContainers to be used in the Pod. |  |  |
| `sidecarContainers` _[Container](#container) array_ | SidecarContainers to be used in the Pod. |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ | Volumes to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints to be used in the Pod. |  |  |


#### PrimaryGalera



PrimaryGalera is the Galera configuration for the primary node.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podIndex` _integer_ | PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover. |  |  |
| `automaticFailover` _boolean_ | AutomaticFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover. |  |  |


#### PrimaryReplication



PrimaryReplication is the replication configuration for the primary node.



_Appears in:_
- [Replication](#replication)
- [ReplicationSpec](#replicationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podIndex` _integer_ | PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover. |  |  |
| `automaticFailover` _boolean_ | AutomaticFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover. |  |  |


#### ReplicaReplication



ReplicaReplication is the replication configuration for the replica nodes.



_Appears in:_
- [Replication](#replication)
- [ReplicationSpec](#replicationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `waitPoint` _[WaitPoint](#waitpoint)_ | WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine.<br />More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point. |  | Enum: [AfterSync AfterCommit] <br /> |
| `gtid` _[Gtid](#gtid)_ | Gtid indicates which Global Transaction ID should be used when connecting a replica to the master.<br />See: https://mariadb.com/kb/en/gtid/#using-current_pos-vs-slave_pos. |  | Enum: [CurrentPos SlavePos] <br /> |
| `replPasswordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | ReplPasswordSecretKeyRef provides a reference to the Secret to use as password for the replication user. |  |  |
| `connectionTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | ConnectionTimeout to be used when the replica connects to the primary. |  |  |
| `connectionRetries` _integer_ | ConnectionRetries to be used when the replica connects to the primary. |  |  |
| `syncTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | SyncTimeout defines the timeout for a replica to be synced with the primary when performing a primary switchover.<br />If the timeout is reached, the replica GTID will be reset and the switchover will continue. |  |  |


#### Replication



Replication allows you to enable single-master HA via semi-synchronours replication in your MariaDB cluster.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `primary` _[PrimaryReplication](#primaryreplication)_ | Primary is the replication configuration for the primary node. |  |  |
| `replica` _[ReplicaReplication](#replicareplication)_ | ReplicaReplication is the replication configuration for the replica nodes. |  |  |
| `syncBinlog` _boolean_ | SyncBinlog indicates whether the binary log should be synchronized to the disk after every event.<br />It trades off performance for consistency.<br />See: https://mariadb.com/kb/en/replication-and-binary-log-system-variables/#sync_binlog. |  |  |
| `probesEnabled` _boolean_ | ProbesEnabled indicates to use replication specific liveness and readiness probes.<br />This probes check that the primary can receive queries and that the replica has the replication thread running. |  |  |
| `enabled` _boolean_ | Enabled is a flag to enable Replication. |  |  |


#### ReplicationSpec



ReplicationSpec is the Replication desired state specification.



_Appears in:_
- [Replication](#replication)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `primary` _[PrimaryReplication](#primaryreplication)_ | Primary is the replication configuration for the primary node. |  |  |
| `replica` _[ReplicaReplication](#replicareplication)_ | ReplicaReplication is the replication configuration for the replica nodes. |  |  |
| `syncBinlog` _boolean_ | SyncBinlog indicates whether the binary log should be synchronized to the disk after every event.<br />It trades off performance for consistency.<br />See: https://mariadb.com/kb/en/replication-and-binary-log-system-variables/#sync_binlog. |  |  |
| `probesEnabled` _boolean_ | ProbesEnabled indicates to use replication specific liveness and readiness probes.<br />This probes check that the primary can receive queries and that the replica has the replication thread running. |  |  |


#### ReplicationState

_Underlying type:_ _string_





_Appears in:_
- [ReplicationStatus](#replicationstatus)



#### Restore



Restore is the Schema for the restores API. It is used to define restore jobs and its restoration source.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Restore` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RestoreSpec](#restorespec)_ |  |  |  |


#### RestoreSource



RestoreSource defines a source for restoring a MariaDB.



_Appears in:_
- [BootstrapFrom](#bootstrapfrom)
- [RestoreSpec](#restorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backupRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | BackupRef is a reference to a Backup object. It has priority over S3 and Volume. |  |  |
| `s3` _[S3](#s3)_ | S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume. |  |  |
| `volume` _[VolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumesource-v1-core)_ | Volume is a Kubernetes Volume object that contains a backup. |  |  |
| `targetRecoveryTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.<br />It is used to determine the closest restoration source in time. |  |  |


#### RestoreSpec



RestoreSpec defines the desired state of restore



_Appears in:_
- [Restore](#restore)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `backupRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | BackupRef is a reference to a Backup object. It has priority over S3 and Volume. |  |  |
| `s3` _[S3](#s3)_ | S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume. |  |  |
| `volume` _[VolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumesource-v1-core)_ | Volume is a Kubernetes Volume object that contains a backup. |  |  |
| `targetRecoveryTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#time-v1-meta)_ | TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.<br />It is used to determine the closest restoration source in time. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: {} <br /> |
| `database` _string_ | Database defines the logical database to be restored. If not provided, all databases available in the backup are restored.<br />IMPORTANT: The database must previously exist. |  |  |
| `logLevel` _string_ | LogLevel to be used n the Backup Job. It defaults to 'info'. | info |  |
| `backoffLimit` _integer_ | BackoffLimit defines the maximum number of attempts to successfully perform a Backup. | 5 |  |
| `restartPolicy` _[RestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#restartpolicy-v1-core)_ | RestartPolicy to be added to the Backup Job. | OnFailure | Enum: [Always OnFailure Never] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |


#### S3







_Appears in:_
- [BackupStorage](#backupstorage)
- [BootstrapFrom](#bootstrapfrom)
- [RestoreSource](#restoresource)
- [RestoreSpec](#restorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucket` _string_ | Bucket is the name Name of the bucket to store backups. |  | Required: {} <br /> |
| `endpoint` _string_ | Endpoint is the S3 API endpoint without scheme. |  | Required: {} <br /> |
| `region` _string_ | Region is the S3 region name to use. |  |  |
| `prefix` _string_ | Prefix indicates a folder/subfolder in the bucket. For example: mariadb/ or mariadb/backups. A trailing slash '/' is added if not provided. |  |  |
| `accessKeyIdSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | AccessKeyIdSecretKeyRef is a reference to a Secret key containing the S3 access key id. |  | Required: {} <br /> |
| `secretAccessKeySecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | AccessKeyIdSecretKeyRef is a reference to a Secret key containing the S3 secret key. |  | Required: {} <br /> |
| `sessionTokenSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | SessionTokenSecretKeyRef is a reference to a Secret key containing the S3 session token. |  |  |
| `tls` _[TLS](#tls)_ | TLS provides the configuration required to establish TLS connections with S3. |  |  |


#### SQLTemplate



SQLTemplate defines a template to customize SQL objects.



_Appears in:_
- [DatabaseSpec](#databasespec)
- [GrantSpec](#grantspec)
- [UserSpec](#userspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |  |  |


#### SST

_Underlying type:_ _string_

SST is the Snapshot State Transfer used when new Pods join the cluster.
More info: https://galeracluster.com/library/documentation/sst.html.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)



#### Schedule



Schedule contains parameters to define a schedule



_Appears in:_
- [BackupSpec](#backupspec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cron` _string_ | Cron is a cron expression that defines the schedule. |  | Required: {} <br /> |
| `suspend` _boolean_ | Suspend defines whether the schedule is active or not. | false |  |


#### SecretTemplate



SecretTemplate defines a template to customize Secret objects.



_Appears in:_
- [ConnectionSpec](#connectionspec)
- [ConnectionTemplate](#connectiontemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `key` _string_ | Key to be used in the Secret. |  |  |
| `format` _string_ | Format to be used in the Secret. |  |  |
| `usernameKey` _string_ | UsernameKey to be used in the Secret. |  |  |
| `passwordKey` _string_ | PasswordKey to be used in the Secret. |  |  |
| `hostKey` _string_ | HostKey to be used in the Secret. |  |  |
| `portKey` _string_ | PortKey to be used in the Secret. |  |  |
| `databaseKey` _string_ | DatabaseKey to be used in the Secret. |  |  |


#### ServiceMonitor



ServiceMonitor defines a prometheus ServiceMonitor object.



_Appears in:_
- [MariadbMetrics](#mariadbmetrics)
- [MaxScaleMetrics](#maxscalemetrics)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `prometheusRelease` _string_ | PrometheusRelease is the release label to add to the ServiceMonitor object. |  |  |
| `jobLabel` _string_ | JobLabel to add to the ServiceMonitor object. |  |  |
| `interval` _string_ | Interval for scraping metrics. |  |  |
| `scrapeTimeout` _string_ | ScrapeTimeout defines the timeout for scraping metrics. |  |  |


#### ServiceRouter

_Underlying type:_ _string_

ServiceRouter defines the type of service router.



_Appears in:_
- [MaxScaleService](#maxscaleservice)



#### ServiceTemplate



ServiceTemplate defines a template to customize Service objects.



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ | Type is the Service type. One of `ClusterIP`, `NodePort` or `LoadBalancer`. If not defined, it defaults to `ClusterIP`. | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br /> |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `loadBalancerIP` _string_ | LoadBalancerIP Service field. |  |  |
| `loadBalancerSourceRanges` _string array_ | LoadBalancerSourceRanges Service field. |  |  |
| `externalTrafficPolicy` _[ServiceExternalTrafficPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#serviceexternaltrafficpolicy-v1-core)_ | ExternalTrafficPolicy Service field. |  |  |
| `sessionAffinity` _[ServiceAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#serviceaffinity-v1-core)_ | SessionAffinity Service field. |  |  |
| `allocateLoadBalancerNodePorts` _boolean_ | AllocateLoadBalancerNodePorts Service field. |  |  |


#### SqlJob



SqlJob is the Schema for the sqljobs API. It is used to run sql scripts as jobs.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `SqlJob` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[SqlJobSpec](#sqljobspec)_ |  |  |  |


#### SqlJobSpec



SqlJobSpec defines the desired state of SqlJob



_Appears in:_
- [SqlJob](#sqljob)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: {} <br /> |
| `schedule` _[Schedule](#schedule)_ | Schedule defines when the SqlJob will be executed. |  | Required: {} <br /> |
| `username` _string_ | Username to be impersonated when executing the SqlJob. |  | Required: {} <br /> |
| `passwordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | UserPasswordSecretKeyRef is a reference to the impersonated user's password to be used when executing the SqlJob. |  | Required: {} <br /> |
| `database` _string_ | Username to be used when executing the SqlJob. |  |  |
| `dependsOn` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ | DependsOn defines dependencies with other SqlJob objectecs. |  |  |
| `sql` _string_ | Sql is the script to be executed by the SqlJob. |  |  |
| `sqlConfigMapKeyRef` _[ConfigMapKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapkeyselector-v1-core)_ | SqlConfigMapKeyRef is a reference to a ConfigMap containing the Sql script.<br />It is defaulted to a ConfigMap with the contents of the Sql field. |  |  |
| `backoffLimit` _integer_ | BackoffLimit defines the maximum number of attempts to successfully execute a SqlJob. | 5 |  |
| `restartPolicy` _[RestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#restartpolicy-v1-core)_ | RestartPolicy to be added to the SqlJob Pod. | OnFailure | Enum: [Always OnFailure Never] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |


#### Storage



Storage defines the storage options to be used for provisioning the PVCs mounted by MariaDB.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ephemeral` _boolean_ | Ephemeral indicates whether to use ephemeral storage in the PVCs. It is only compatible with non HA MariaDBs. |  |  |
| `size` _[Quantity](#quantity)_ | Size of the PVCs to be mounted by MariaDB. Required if not provided in 'VolumeClaimTemplate'. It superseeds the storage size specified in 'VolumeClaimTemplate'. |  |  |
| `storageClassName` _string_ | StorageClassName to be used to provision the PVCS. It superseeds the 'StorageClassName' specified in 'VolumeClaimTemplate'.<br />If not provided, the default 'StorageClass' configured in the cluster is used. |  |  |
| `resizeInUseVolumes` _boolean_ | ResizeInUseVolumes indicates whether the PVCs can be resized. The 'StorageClassName' used should have 'allowVolumeExpansion' set to 'true' to allow resizing.<br />It defaults to true. |  |  |
| `waitForVolumeResize` _boolean_ | WaitForVolumeResize indicates whether to wait for the PVCs to be resized before marking the MariaDB object as ready. This will block other operations such as cluster recovery while the resize is in progress.<br />It defaults to true. |  |  |
| `volumeClaimTemplate` _[VolumeClaimTemplate](#volumeclaimtemplate)_ | VolumeClaimTemplate provides a template to define the PVCs. |  |  |


#### SuspendTemplate



SuspendTemplate indicates whether the current resource should be suspended or not. Feature flag --feature-maxscale-suspend is required in the controller to enable this.



_Appears in:_
- [MaxScaleListener](#maxscalelistener)
- [MaxScaleMonitor](#maxscalemonitor)
- [MaxScaleService](#maxscaleservice)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not. Feature flag --feature-maxscale-suspend is required in the controller to enable this. |  |  |


#### TLS







_Appears in:_
- [S3](#s3)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable TLS. |  |  |
| `caSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | CASecretKeyRef is a reference to a Secret key containing a CA bundle in PEM format used to establish TLS connections with S3.<br />By default, the system trust chain will be used, but you can use this field to add more CAs to the bundle. |  |  |


#### UpdateStrategy



UpdateStrategy defines how a MariaDB resource is updated.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[UpdateType](#updatetype)_ | Type defines the type of updates. One of `ReplicasFirstPrimaryLast`, `RollingUpdate` or `OnDelete`. If not defined, it defaults to `ReplicasFirstPrimaryLast`. | ReplicasFirstPrimaryLast | Enum: [ReplicasFirstPrimaryLast RollingUpdate OnDelete] <br /> |
| `rollingUpdate` _[RollingUpdateStatefulSetStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#rollingupdatestatefulsetstrategy-v1-apps)_ | RollingUpdate defines parameters for the RollingUpdate type. |  |  |


#### UpdateType

_Underlying type:_ _string_

UpdateType defines the type of update for a MariaDB resource.



_Appears in:_
- [UpdateStrategy](#updatestrategy)



#### User



User is the Schema for the users API.  It is used to define grants as if you were running a 'CREATE USER' statement.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `User` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[UserSpec](#userspec)_ |  |  |  |


#### UserSpec



UserSpec defines the desired state of User



_Appears in:_
- [User](#user)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: {} <br /> |
| `passwordSecretKeyRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ | PasswordSecretKeyRef is a reference to the password to be used by the User.<br />If not provided, the account will be locked and the password will expire.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `maxUserConnections` _integer_ | MaxUserConnections defines the maximum number of connections that the User can establish. | 10 |  |
| `name` _string_ | Name overrides the default name provided by metadata.name. |  | MaxLength: 80 <br /> |
| `host` _string_ | Host related to the User. |  | MaxLength: 255 <br /> |


#### VolumeClaimTemplate



VolumeClaimTemplate defines a template to customize PVC objects.



_Appears in:_
- [GaleraConfig](#galeraconfig)
- [MaxScaleConfig](#maxscaleconfig)
- [Storage](#storage)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeaccessmode-v1-core) array_ | accessModes contains the desired access modes the volume should have.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1 |  |  |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#labelselector-v1-meta)_ | selector is a label query over volumes to consider for binding. |  |  |
| `resources` _[VolumeResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumeresourcerequirements-v1-core)_ | resources represents the minimum resources the volume should have.<br />If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements<br />that are lower than previous value but must still be higher than capacity recorded in the<br />status field of the claim.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources |  |  |
| `volumeName` _string_ | volumeName is the binding reference to the PersistentVolume backing this claim. |  |  |
| `storageClassName` _string_ | storageClassName is the name of the StorageClass required by the claim.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1 |  |  |
| `volumeMode` _[PersistentVolumeMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumemode-v1-core)_ | volumeMode defines what type of volume is required by the claim.<br />Value of Filesystem is implied when not included in claim spec. |  |  |
| `dataSource` _[TypedLocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#typedlocalobjectreference-v1-core)_ | dataSource field can be used to specify either:<br />* An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot)<br />* An existing PVC (PersistentVolumeClaim)<br />If the provisioner or an external controller can support the specified data source,<br />it will create a new volume based on the contents of the specified data source.<br />When the AnyVolumeDataSource feature gate is enabled, dataSource contents will be copied to dataSourceRef,<br />and dataSourceRef contents will be copied to dataSource when dataSourceRef.namespace is not specified.<br />If the namespace is specified, then dataSourceRef will not be copied to dataSource. |  |  |
| `dataSourceRef` _[TypedObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#typedobjectreference-v1-core)_ | dataSourceRef specifies the object from which to populate the volume with data, if a non-empty<br />volume is desired. This may be any object from a non-empty API group (non<br />core object) or a PersistentVolumeClaim object.<br />When this field is specified, volume binding will only succeed if the type of<br />the specified object matches some installed volume populator or dynamic<br />provisioner.<br />This field will replace the functionality of the dataSource field and as such<br />if both fields are non-empty, they must have the same value. For backwards<br />compatibility, when namespace isn't specified in dataSourceRef,<br />both fields (dataSource and dataSourceRef) will be set to the same<br />value automatically if one of them is empty and the other is non-empty.<br />When namespace is specified in dataSourceRef,<br />dataSource isn't set to the same value and must be empty.<br />There are three important differences between dataSource and dataSourceRef:<br />* While dataSource only allows two specific types of objects, dataSourceRef<br />  allows any non-core object, as well as PersistentVolumeClaim objects.<br />* While dataSource ignores disallowed values (dropping them), dataSourceRef<br />  preserves all values, and generates an error if a disallowed value is<br />  specified.<br />* While dataSource only allows local objects, dataSourceRef allows objects<br />  in any namespaces.<br />(Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled.<br />(Alpha) Using the namespace field of dataSourceRef requires the CrossNamespaceVolumeDataSource feature gate to be enabled. |  |  |
| `volumeAttributesClassName` _string_ | volumeAttributesClassName may be used to set the VolumeAttributesClass used by this claim.<br />If specified, the CSI driver will create or update the volume with the attributes defined<br />in the corresponding VolumeAttributesClass. This has a different purpose than storageClassName,<br />it can be changed after the claim is created. An empty string value means that no VolumeAttributesClass<br />will be applied to the claim but it's not allowed to reset this field to empty string once it is set.<br />If unspecified and the PersistentVolumeClaim is unbound, the default VolumeAttributesClass<br />will be set by the persistentvolume controller if it exists.<br />If the resource referred to by volumeAttributesClass does not exist, this PersistentVolumeClaim will be<br />set to a Pending state, as reflected by the modifyVolumeStatus field, until such as a resource<br />exists.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#volumeattributesclass<br />(Alpha) Using this field requires the VolumeAttributesClass feature gate to be enabled. |  |  |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |


#### WaitPoint

_Underlying type:_ _string_

WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine.
More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.



_Appears in:_
- [ReplicaReplication](#replicareplication)



