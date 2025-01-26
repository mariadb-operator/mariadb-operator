# API Reference

## Packages
- [k8s.mariadb.com/v1alpha1](#k8smariadbcomv1alpha1)


## k8s.mariadb.com/v1alpha1

Package v1alpha1 contains API Schema definitions for the v1alpha1 API group

nolint:lll

nolint:lll

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



#### Affinity



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core.



_Appears in:_
- [AffinityConfig](#affinityconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podAntiAffinity` _[PodAntiAffinity](#podantiaffinity)_ |  |  |  |
| `nodeAffinity` _[NodeAffinity](#nodeaffinity)_ |  |  |  |


#### AffinityConfig



AffinityConfig defines policies to schedule Pods in Nodes.



_Appears in:_
- [BackupSpec](#backupspec)
- [Exporter](#exporter)
- [Job](#job)
- [JobPodTemplate](#jobpodtemplate)
- [MariaDBSpec](#mariadbspec)
- [MaxScalePodTemplate](#maxscalepodtemplate)
- [MaxScaleSpec](#maxscalespec)
- [PodTemplate](#podtemplate)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podAntiAffinity` _[PodAntiAffinity](#podantiaffinity)_ |  |  |  |
| `nodeAffinity` _[NodeAffinity](#nodeaffinity)_ |  |  |  |
| `antiAffinityEnabled` _boolean_ | AntiAffinityEnabled configures PodAntiAffinity so each Pod is scheduled in a different Node, enabling HA.<br />Make sure you have at least as many Nodes available as the replicas to not end up with unscheduled Pods. |  |  |


#### Backup



Backup is the Schema for the backups API. It is used to define backup jobs and its storage.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Backup` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[BackupSpec](#backupspec)_ |  |  |  |


#### BackupSpec



BackupSpec defines the desired state of Backup



_Appears in:_
- [Backup](#backup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](#localobjectreference) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](#podsecuritycontext)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `successfulJobsHistoryLimit` _integer_ | SuccessfulJobsHistoryLimit defines the maximum number of successful Jobs to be displayed. |  | Minimum: 0 <br /> |
| `failedJobsHistoryLimit` _integer_ | FailedJobsHistoryLimit defines the maximum number of failed Jobs to be displayed. |  | Minimum: 0 <br /> |
| `timeZone` _string_ | TimeZone defines the timezone associated with the cron expression. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: \{\} <br /> |
| `compression` _[CompressAlgorithm](#compressalgorithm)_ | Compression algorithm to be used in the Backup. |  | Enum: [none bzip2 gzip] <br /> |
| `stagingStorage` _[BackupStagingStorage](#backupstagingstorage)_ | StagingStorage defines the temporary storage used to keep external backups (i.e. S3) while they are being processed.<br />It defaults to an emptyDir volume, meaning that the backups will be temporarily stored in the node where the Backup Job is scheduled.<br />The staging area gets cleaned up after each backup is completed, consider this for sizing it appropriately. |  |  |
| `storage` _[BackupStorage](#backupstorage)_ | Storage defines the final storage for backups. |  | Required: \{\} <br /> |
| `schedule` _[Schedule](#schedule)_ | Schedule defines when the Backup will be taken. |  |  |
| `maxRetention` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | MaxRetention defines the retention policy for backups. Old backups will be cleaned up by the Backup Job.<br />It defaults to 30 days. |  |  |
| `databases` _string array_ | Databases defines the logical databases to be backed up. If not provided, all databases are backed up. |  |  |
| `ignoreGlobalPriv` _boolean_ | IgnoreGlobalPriv indicates to ignore the mysql.global_priv in backups.<br />If not provided, it will default to true when the referred MariaDB instance has Galera enabled and otherwise to false.<br />See: https://github.com/mariadb-operator/mariadb-operator/issues/556 |  |  |
| `logLevel` _string_ | LogLevel to be used n the Backup Job. It defaults to 'info'. | info |  |
| `backoffLimit` _integer_ | BackoffLimit defines the maximum number of attempts to successfully take a Backup. |  |  |
| `restartPolicy` _[RestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#restartpolicy-v1-core)_ | RestartPolicy to be added to the Backup Pod. | OnFailure | Enum: [Always OnFailure Never] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |


#### BackupStagingStorage



BackupStagingStorage defines the temporary storage used to keep external backups (i.e. S3) while they are being processed.



_Appears in:_
- [BackupSpec](#backupspec)
- [BootstrapFrom](#bootstrapfrom)
- [RestoreSource](#restoresource)
- [RestoreSpec](#restorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `persistentVolumeClaim` _[PersistentVolumeClaimSpec](#persistentvolumeclaimspec)_ | PersistentVolumeClaim is a Kubernetes PVC specification. |  |  |
| `volume` _[StorageVolumeSource](#storagevolumesource)_ | Volume is a Kubernetes volume specification. |  |  |


#### BackupStorage



BackupStorage defines the final storage for backups.



_Appears in:_
- [BackupSpec](#backupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `s3` _[S3](#s3)_ | S3 defines the configuration to store backups in a S3 compatible storage. |  |  |
| `persistentVolumeClaim` _[PersistentVolumeClaimSpec](#persistentvolumeclaimspec)_ | PersistentVolumeClaim is a Kubernetes PVC specification. |  |  |
| `volume` _[StorageVolumeSource](#storagevolumesource)_ | Volume is a Kubernetes volume specification. |  |  |


#### BasicAuth



KubernetesAuth refers to the basic authentication mechanism utilized for establishing a connection from the operator to the agent.



_Appears in:_
- [GaleraAgent](#galeraagent)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable BasicAuth |  |  |
| `username` _string_ | Username to be used for basic authentication |  |  |
| `passwordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | PasswordSecretKeyRef to be used for basic authentication |  |  |


#### BootstrapFrom



BootstrapFrom defines a source to bootstrap MariaDB from.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backupRef` _[LocalObjectReference](#localobjectreference)_ | BackupRef is a reference to a Backup object. It has priority over S3 and Volume. |  |  |
| `s3` _[S3](#s3)_ | S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume. |  |  |
| `volume` _[StorageVolumeSource](#storagevolumesource)_ | Volume is a Kubernetes Volume object that contains a backup. |  |  |
| `targetRecoveryTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ | TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.<br />It is used to determine the closest restoration source in time. |  |  |
| `stagingStorage` _[BackupStagingStorage](#backupstagingstorage)_ | StagingStorage defines the temporary storage used to keep external backups (i.e. S3) while they are being processed.<br />It defaults to an emptyDir volume, meaning that the backups will be temporarily stored in the node where the Restore Job is scheduled. |  |  |
| `restoreJob` _[Job](#job)_ | RestoreJob defines additional properties for the Job used to perform the Restore. |  |  |


#### CSIVolumeSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#csivolumesource-v1-core.



_Appears in:_
- [StorageVolumeSource](#storagevolumesource)
- [Volume](#volume)
- [VolumeSource](#volumesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `driver` _string_ |  |  |  |
| `readOnly` _boolean_ |  |  |  |
| `fsType` _string_ |  |  |  |
| `volumeAttributes` _object (keys:string, values:string)_ |  |  |  |
| `nodePublishSecretRef` _[LocalObjectReference](#localobjectreference)_ |  |  |  |


#### CleanupPolicy

_Underlying type:_ _string_

CleanupPolicy defines the behavior for cleaning up a resource.



_Appears in:_
- [DatabaseSpec](#databasespec)
- [GrantSpec](#grantspec)
- [SQLTemplate](#sqltemplate)
- [UserSpec](#userspec)

| Field | Description |
| --- | --- |
| `Skip` | CleanupPolicySkip indicates that the resource will NOT be deleted from the database after the CR is deleted.<br /> |
| `Delete` | CleanupPolicyDelete indicates that the resource will be deleted from the database after the CR is deleted.<br /> |


#### CompressAlgorithm

_Underlying type:_ _string_

CompressAlgorithm defines the compression algorithm for a Backup resource.



_Appears in:_
- [BackupSpec](#backupspec)

| Field | Description |
| --- | --- |
| `none` | No compression<br /> |
| `bzip2` | Bzip2 compression. Good compression ratio, but slower compression/decompression speed compared to gzip.<br /> |
| `gzip` | Gzip compression. Good compression/decompression speed, but worse compression ratio compared to bzip2.<br /> |


#### ConfigMapKeySelector



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#configmapkeyselector-v1-core.



_Appears in:_
- [EnvVarSource](#envvarsource)
- [MariaDBSpec](#mariadbspec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `key` _string_ |  |  |  |


#### ConfigMapVolumeSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#configmapvolumesource-v1-core.



_Appears in:_
- [Volume](#volume)
- [VolumeSource](#volumesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `defaultMode` _integer_ |  |  |  |


#### Connection



Connection is the Schema for the connections API. It is used to configure connection strings for the applications connecting to MariaDB.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Connection` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
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
| `maxScaleRef` _[ObjectReference](#objectreference)_ | MaxScaleRef is a reference to the MaxScale to connect to. Either MariaDBRef or MaxScaleRef must be provided. |  |  |
| `username` _string_ | Username to use for configuring the Connection. |  | Required: \{\} <br /> |
| `passwordSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | PasswordSecretKeyRef is a reference to the password to use for configuring the Connection.<br />Either passwordSecretKeyRef or tlsClientCertSecretRef must be provided as client credentials.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `tlsClientCertSecretRef` _[LocalObjectReference](#localobjectreference)_ | TLSClientCertSecretRef is a reference to a Kubernetes TLS Secret used as authentication when checking the connection health.<br />Either passwordSecretKeyRef or tlsClientCertSecretRef must be provided as client credentials.<br />If not provided, the client certificate provided by the referred MariaDB is used if TLS is enabled.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the client certificate. |  |  |
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
- [MariaDBSpec](#mariadbspec)
- [PodTemplate](#podtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name to be given to the container. |  |  |
| `image` _string_ | Image name to be used by the container. The supported format is `<image>:<tag>`. |  | Required: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](#envvar) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `volumeMounts` _[VolumeMount](#volumemount) array_ | VolumeMounts to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |


#### ContainerTemplate



ContainerTemplate defines a template to configure Container objects.



_Appears in:_
- [GaleraAgent](#galeraagent)
- [GaleraInit](#galerainit)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](#envvar) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](#envfromsource) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](#volumemount) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](#probe)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](#probe)_ | ReadinessProbe to be used in the Container. |  |  |
| `startupProbe` _[Probe](#probe)_ | StartupProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |


#### CooperativeMonitoring

_Underlying type:_ _string_

CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors.
See: https://mariadb.com/docs/server/architecture/components/maxscale/monitors/mariadbmon/use-cooperative-locking-ha-maxscale-mariadb-monitor/



_Appears in:_
- [MaxScaleMonitor](#maxscalemonitor)

| Field | Description |
| --- | --- |
| `majority_of_all` | CooperativeMonitoringMajorityOfAll requires a lock from the majority of the MariaDB servers, even the ones that are down.<br /> |
| `majority_of_running` | CooperativeMonitoringMajorityOfRunning requires a lock from the majority of the MariaDB servers.<br /> |


#### CronJobTemplate



CronJobTemplate defines parameters for configuring CronJob objects.



_Appears in:_
- [BackupSpec](#backupspec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `successfulJobsHistoryLimit` _integer_ | SuccessfulJobsHistoryLimit defines the maximum number of successful Jobs to be displayed. |  | Minimum: 0 <br /> |
| `failedJobsHistoryLimit` _integer_ | FailedJobsHistoryLimit defines the maximum number of failed Jobs to be displayed. |  | Minimum: 0 <br /> |
| `timeZone` _string_ | TimeZone defines the timezone associated with the cron expression. |  |  |


#### Database



Database is the Schema for the databases API. It is used to define a logical database as if you were running a 'CREATE DATABASE' statement.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Database` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[DatabaseSpec](#databasespec)_ |  |  |  |


#### DatabaseSpec



DatabaseSpec defines the desired state of Database



_Appears in:_
- [Database](#database)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |  |  |
| `cleanupPolicy` _[CleanupPolicy](#cleanuppolicy)_ | CleanupPolicy defines the behavior for cleaning up a SQL resource. |  | Enum: [Skip Delete] <br /> |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: \{\} <br /> |
| `characterSet` _string_ | CharacterSet to use in the Database. | utf8 |  |
| `collate` _string_ | Collate to use in the Database. | utf8_general_ci |  |
| `name` _string_ | Name overrides the default Database name provided by metadata.name. |  | MaxLength: 80 <br /> |


#### EmptyDirVolumeSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#emptydirvolumesource-v1-core.



_Appears in:_
- [StorageVolumeSource](#storagevolumesource)
- [Volume](#volume)
- [VolumeSource](#volumesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `medium` _[StorageMedium](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#storagemedium-v1-core)_ |  |  |  |
| `sizeLimit` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#quantity-resource-api)_ |  |  |  |


#### EnvFromSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envfromsource-v1-core.



_Appears in:_
- [ContainerTemplate](#containertemplate)
- [GaleraAgent](#galeraagent)
- [GaleraInit](#galerainit)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `prefix` _string_ |  |  |  |
| `configMapRef` _[LocalObjectReference](#localobjectreference)_ |  |  |  |
| `secretRef` _[LocalObjectReference](#localobjectreference)_ |  |  |  |


#### EnvVar



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvarsource-v1-core.



_Appears in:_
- [Container](#container)
- [ContainerTemplate](#containertemplate)
- [GaleraAgent](#galeraagent)
- [GaleraInit](#galerainit)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the environment variable. Must be a C_IDENTIFIER. |  |  |
| `value` _string_ |  |  |  |
| `valueFrom` _[EnvVarSource](#envvarsource)_ |  |  |  |


#### EnvVarSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvarsource-v1-core.



_Appears in:_
- [EnvVar](#envvar)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `fieldRef` _[ObjectFieldSelector](#objectfieldselector)_ |  |  |  |
| `configMapKeyRef` _[ConfigMapKeySelector](#configmapkeyselector)_ |  |  |  |
| `secretKeyRef` _[SecretKeySelector](#secretkeyselector)_ |  |  |  |


#### ExecAction



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#execaction-v1-core.



_Appears in:_
- [Probe](#probe)
- [ProbeHandler](#probehandler)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ |  |  |  |


#### Exporter



Exporter defines a metrics exporter container.



_Appears in:_
- [MariadbMetrics](#mariadbmetrics)
- [MaxScaleMetrics](#maxscalemetrics)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image name to be used as metrics exporter. The supported format is `<image>:<tag>`.<br />Only mysqld-exporter >= v0.15.0 is supported: https://github.com/prometheus/mysqld_exporter |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `imagePullSecrets` _[LocalObjectReference](#localobjectreference) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `port` _integer_ | Port where the exporter will be listening for connections. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds container-level security attributes. |  |  |
| `podSecurityContext` _[PodSecurityContext](#podsecuritycontext)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |


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
| `initContainer` _[GaleraInit](#galerainit)_ | InitContainer is an init container that runs in the MariaDB Pod and co-operates with mariadb-operator. |  |  |
| `initJob` _[GaleraInitJob](#galerainitjob)_ | InitJob defines a Job that co-operates with mariadb-operator by performing initialization tasks. |  |  |
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
| `env` _[EnvVar](#envvar) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](#envfromsource) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](#volumemount) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](#probe)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](#probe)_ | ReadinessProbe to be used in the Container. |  |  |
| `startupProbe` _[Probe](#probe)_ | StartupProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `image` _string_ | Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `port` _integer_ | Port where the agent will be listening for API connections. |  |  |
| `probePort` _integer_ | Port where the agent will be listening for probe connections. |  |  |
| `kubernetesAuth` _[KubernetesAuth](#kubernetesauth)_ | KubernetesAuth to be used by the agent container |  |  |
| `basicAuth` _[BasicAuth](#basicauth)_ | BasicAuth to be used by the agent container |  |  |
| `gracefulShutdownTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | GracefulShutdownTimeout is the time we give to the agent container in order to gracefully terminate in-flight requests. |  |  |


#### GaleraConfig



GaleraConfig defines storage options for the Galera configuration files.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `reuseStorageVolume` _boolean_ | ReuseStorageVolume indicates that storage volume used by MariaDB should be reused to store the Galera configuration files.<br />It defaults to false, which implies that a dedicated volume for the Galera configuration files is provisioned. |  |  |
| `volumeClaimTemplate` _[VolumeClaimTemplate](#volumeclaimtemplate)_ | VolumeClaimTemplate is a template for the PVC that will contain the Galera configuration files shared between the InitContainer, Agent and MariaDB. |  |  |


#### GaleraInit



GaleraInit is an init container that runs in the MariaDB Pod and co-operates with mariadb-operator.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](#envvar) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](#envfromsource) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](#volumemount) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](#probe)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](#probe)_ | ReadinessProbe to be used in the Container. |  |  |
| `startupProbe` _[Probe](#probe)_ | StartupProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `image` _string_ | Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`. |  | Required: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |


#### GaleraInitJob



GaleraInitJob defines a Job used to be used to initialize the Galera cluster.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |


#### GaleraRecovery



GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
More info: https://galeracluster.com/library/documentation/crash-recovery.html.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable GaleraRecovery. |  |  |
| `minClusterSize` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ | MinClusterSize is the minimum number of replicas to consider the cluster healthy. It can be either a number of replicas (1) or a percentage (50%).<br />If Galera consistently reports less replicas than this value for the given 'ClusterHealthyTimeout' interval, a cluster recovery is iniated.<br />It defaults to '1' replica, and it is highly recommendeded to keep this value at '1' in most cases.<br />If set to more than one replica, the cluster recovery process may restart the healthy replicas as well. |  |  |
| `clusterMonitorInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | ClusterMonitorInterval represents the interval used to monitor the Galera cluster health. |  |  |
| `clusterHealthyTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | ClusterHealthyTimeout represents the duration at which a Galera cluster, that consistently failed health checks,<br />is considered unhealthy, and consequently the Galera recovery process will be initiated by the operator. |  |  |
| `clusterBootstrapTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | ClusterBootstrapTimeout is the time limit for bootstrapping a cluster.<br />Once this timeout is reached, the Galera recovery state is reset and a new cluster bootstrap will be attempted. |  |  |
| `clusterUpscaleTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | ClusterUpscaleTimeout represents the maximum duration for upscaling the cluster's StatefulSet during the recovery process. |  |  |
| `clusterDownscaleTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | ClusterDownscaleTimeout represents the maximum duration for downscaling the cluster's StatefulSet during the recovery process. |  |  |
| `podRecoveryTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | PodRecoveryTimeout is the time limit for recevorying the sequence of a Pod during the cluster recovery. |  |  |
| `podSyncTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | PodSyncTimeout is the time limit for a Pod to join the cluster after having performed a cluster bootstrap during the cluster recovery. |  |  |
| `forceClusterBootstrapInPod` _string_ | ForceClusterBootstrapInPod allows you to manually initiate the bootstrap process in a specific Pod.<br />IMPORTANT: Use this option only in exceptional circumstances. Not selecting the Pod with the highest sequence number may result in data loss.<br />IMPORTANT: Ensure you unset this field after completing the bootstrap to allow the operator to choose the appropriate Pod to bootstrap from in an event of cluster recovery. |  |  |
| `job` _[GaleraRecoveryJob](#galerarecoveryjob)_ | Job defines a Job that co-operates with mariadb-operator by performing the Galera cluster recovery . |  |  |


#### GaleraRecoveryJob



GaleraRecoveryJob defines a Job used to be used to recover the Galera cluster.



_Appears in:_
- [GaleraRecovery](#galerarecovery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `podAffinity` _boolean_ | PodAffinity indicates whether the recovery Jobs should run in the same Node as the MariaDB Pods. It defaults to true. |  |  |


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
| `initContainer` _[GaleraInit](#galerainit)_ | InitContainer is an init container that runs in the MariaDB Pod and co-operates with mariadb-operator. |  |  |
| `initJob` _[GaleraInitJob](#galerainitjob)_ | InitJob defines a Job that co-operates with mariadb-operator by performing initialization tasks. |  |  |
| `config` _[GaleraConfig](#galeraconfig)_ | GaleraConfig defines storage options for the Galera configuration files. |  |  |


#### GeneratedSecretKeyRef



GeneratedSecretKeyRef defines a reference to a Secret that can be automatically generated by mariadb-operator if needed.



_Appears in:_
- [BasicAuth](#basicauth)
- [MariaDBSpec](#mariadbspec)
- [MariadbMetrics](#mariadbmetrics)
- [MaxScaleAuth](#maxscaleauth)
- [ReplicaReplication](#replicareplication)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `key` _string_ |  |  |  |
| `generate` _boolean_ | Generate indicates whether the Secret should be generated if the Secret referenced is not present. | false |  |


#### Grant



Grant is the Schema for the grants API. It is used to define grants as if you were running a 'GRANT' statement.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Grant` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[GrantSpec](#grantspec)_ |  |  |  |


#### GrantSpec



GrantSpec defines the desired state of Grant



_Appears in:_
- [Grant](#grant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |  |  |
| `cleanupPolicy` _[CleanupPolicy](#cleanuppolicy)_ | CleanupPolicy defines the behavior for cleaning up a SQL resource. |  | Enum: [Skip Delete] <br /> |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: \{\} <br /> |
| `privileges` _string array_ | Privileges to use in the Grant. |  | MinItems: 1 <br />Required: \{\} <br /> |
| `database` _string_ | Database to use in the Grant. | * |  |
| `table` _string_ | Table to use in the Grant. | * |  |
| `username` _string_ | Username to use in the Grant. |  | Required: \{\} <br /> |
| `host` _string_ | Host to use in the Grant. It can be localhost, an IP or '%'. |  |  |
| `grantOption` _boolean_ | GrantOption to use in the Grant. | false |  |


#### Gtid

_Underlying type:_ _string_

Gtid indicates which Global Transaction ID should be used when connecting a replica to the master.
See: https://mariadb.com/kb/en/gtid/#using-current_pos-vs-slave_pos.



_Appears in:_
- [ReplicaReplication](#replicareplication)

| Field | Description |
| --- | --- |
| `CurrentPos` | GtidCurrentPos indicates the union of gtid_binlog_pos and gtid_slave_pos will be used when replicating from master.<br />This is the default Gtid mode.<br /> |
| `SlavePos` | GtidSlavePos indicates that gtid_slave_pos will be used when replicating from master.<br /> |


#### HTTPGetAction



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#httpgetaction-v1-core.



_Appears in:_
- [Probe](#probe)
- [ProbeHandler](#probehandler)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ |  |  |  |
| `port` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ |  |  |  |
| `host` _string_ |  |  |  |
| `scheme` _[URIScheme](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#urischeme-v1-core)_ |  |  |  |


#### HealthCheck



HealthCheck defines intervals for performing health checks.



_Appears in:_
- [ConnectionSpec](#connectionspec)
- [ConnectionTemplate](#connectiontemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | Interval used to perform health checks. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RetryInterval is the interval used to perform health check retries. |  |  |


#### Job



Job defines a Job used to be used with MariaDB.



_Appears in:_
- [BootstrapFrom](#bootstrapfrom)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
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
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |


#### JobPodTemplate



JobPodTemplate defines a template to configure Container objects that run in a Job.



_Appears in:_
- [BackupSpec](#backupspec)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](#localobjectreference) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](#podsecuritycontext)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
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


#### LabelSelector

_Underlying type:_ _[struct{MatchLabels map[string]string "json:\"matchLabels,omitempty\""; MatchExpressions []LabelSelectorRequirement "json:\"matchExpressions,omitempty\""}](#struct{matchlabels-map[string]string-"json:\"matchlabels,omitempty\"";-matchexpressions-[]labelselectorrequirement-"json:\"matchexpressions,omitempty\""})_

Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta



_Appears in:_
- [PodAffinityTerm](#podaffinityterm)





#### LocalObjectReference



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core.



_Appears in:_
- [BackupSpec](#backupspec)
- [BootstrapFrom](#bootstrapfrom)
- [CSIVolumeSource](#csivolumesource)
- [ConfigMapKeySelector](#configmapkeyselector)
- [ConfigMapVolumeSource](#configmapvolumesource)
- [ConnectionSpec](#connectionspec)
- [EnvFromSource](#envfromsource)
- [Exporter](#exporter)
- [GeneratedSecretKeyRef](#generatedsecretkeyref)
- [JobPodTemplate](#jobpodtemplate)
- [MariaDBSpec](#mariadbspec)
- [MaxScalePodTemplate](#maxscalepodtemplate)
- [MaxScaleSpec](#maxscalespec)
- [MaxScaleTLS](#maxscaletls)
- [PodTemplate](#podtemplate)
- [RestoreSource](#restoresource)
- [RestoreSpec](#restorespec)
- [SecretKeySelector](#secretkeyselector)
- [SqlJobSpec](#sqljobspec)
- [TLS](#tls)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |


#### MariaDB



MariaDB is the Schema for the mariadbs API. It is used to define MariaDB clusters.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `MariaDB` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[MariaDBSpec](#mariadbspec)_ |  |  |  |


#### MariaDBMaxScaleSpec



MariaDBMaxScaleSpec defines a reduced version of MaxScale to be used with the current MariaDB.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable a MaxScale instance to be used with the current MariaDB. |  |  |
| `image` _string_ | Image name to be used by the MaxScale instances. The supported format is `<image>:<tag>`.<br />Only MariaDB official images are supported. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `services` _[MaxScaleService](#maxscaleservice) array_ | Services define how the traffic is forwarded to the MariaDB servers. |  |  |
| `monitor` _[MaxScaleMonitor](#maxscalemonitor)_ | Monitor monitors MariaDB server instances. |  |  |
| `admin` _[MaxScaleAdmin](#maxscaleadmin)_ | Admin configures the admin REST API and GUI. |  |  |
| `config` _[MaxScaleConfig](#maxscaleconfig)_ | Config defines the MaxScale configuration. |  |  |
| `auth` _[MaxScaleAuth](#maxscaleauth)_ | Auth defines the credentials required for MaxScale to connect to MariaDB. |  |  |
| `metrics` _[MaxScaleMetrics](#maxscalemetrics)_ | Metrics configures metrics and how to scrape them. |  |  |
| `tls` _[MaxScaleTLS](#maxscaletls)_ | TLS defines the PKI to be used with MaxScale. |  |  |
| `connection` _[ConnectionTemplate](#connectiontemplate)_ | Connection provides a template to define the Connection for MaxScale. |  |  |
| `replicas` _integer_ | Replicas indicates the number of desired instances. |  |  |
| `podDisruptionBudget` _[PodDisruptionBudget](#poddisruptionbudget)_ | PodDisruptionBudget defines the budget for replica availability. |  |  |
| `updateStrategy` _[StatefulSetUpdateStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#statefulsetupdatestrategy-v1-apps)_ | UpdateStrategy defines the update strategy for the StatefulSet object. |  |  |
| `kubernetesService` _[ServiceTemplate](#servicetemplate)_ | KubernetesService defines a template for a Kubernetes Service object to connect to MaxScale. |  |  |
| `guiKubernetesService` _[ServiceTemplate](#servicetemplate)_ | GuiKubernetesService define a template for a Kubernetes Service object to connect to MaxScale's GUI. |  |  |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |


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
| `name` _string_ |  |  |  |
| `namespace` _string_ |  |  |  |
| `waitForIt` _boolean_ | WaitForIt indicates whether the controller using this reference should wait for MariaDB to be ready. | true |  |


#### MariaDBSpec



MariaDBSpec defines the desired state of MariaDB



_Appears in:_
- [MariaDB](#mariadb)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](#envvar) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](#envfromsource) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](#volumemount) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](#probe)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](#probe)_ | ReadinessProbe to be used in the Container. |  |  |
| `startupProbe` _[Probe](#probe)_ | StartupProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](#localobjectreference) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `initContainers` _[Container](#container) array_ | InitContainers to be used in the Pod. |  |  |
| `sidecarContainers` _[Container](#container) array_ | SidecarContainers to be used in the Pod. |  |  |
| `podSecurityContext` _[PodSecurityContext](#podsecuritycontext)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `volumes` _[Volume](#volume) array_ | Volumes to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](#topologyspreadconstraint) array_ | TopologySpreadConstraints to be used in the Pod. |  |  |
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not.<br />This can be useful for maintenance, as disabling the reconciliation prevents the operator from interfering with user operations during maintenance activities. | false |  |
| `image` _string_ | Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`.<br />Only MariaDB official images are supported. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |
| `rootPasswordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | RootPasswordSecretKeyRef is a reference to a Secret key containing the root password. |  |  |
| `rootEmptyPassword` _boolean_ | RootEmptyPassword indicates if the root password should be empty. Don't use this feature in production, it is only intended for development and test environments. |  |  |
| `database` _string_ | Database is the name of the initial Database. |  |  |
| `username` _string_ | Username is the initial username to be created by the operator once MariaDB is ready.<br />The initial User will have ALL PRIVILEGES in the initial Database. |  |  |
| `passwordSecretKeyRef` _[GeneratedSecretKeyRef](#generatedsecretkeyref)_ | PasswordSecretKeyRef is a reference to a Secret that contains the password to be used by the initial User.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `passwordHashSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | PasswordHashSecretKeyRef is a reference to the password hash to be used by the initial User.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password hash. |  |  |
| `passwordPlugin` _[PasswordPlugin](#passwordplugin)_ | PasswordPlugin is a reference to the password plugin and arguments to be used by the initial User. |  |  |
| `myCnf` _string_ | MyCnf allows to specify the my.cnf file mounted by Mariadb.<br />Updating this field will trigger an update to the Mariadb resource. |  |  |
| `myCnfConfigMapKeyRef` _[ConfigMapKeySelector](#configmapkeyselector)_ | MyCnfConfigMapKeyRef is a reference to the my.cnf config file provided via a ConfigMap.<br />If not provided, it will be defaulted with a reference to a ConfigMap containing the MyCnf field.<br />If the referred ConfigMap is labeled with "k8s.mariadb.com/watch", an update to the Mariadb resource will be triggered when the ConfigMap is updated. |  |  |
| `timeZone` _string_ | TimeZone sets the default timezone. If not provided, it defaults to SYSTEM and the timezone data is not loaded. |  |  |
| `bootstrapFrom` _[BootstrapFrom](#bootstrapfrom)_ | BootstrapFrom defines a source to bootstrap from. |  |  |
| `storage` _[Storage](#storage)_ | Storage defines the storage options to be used for provisioning the PVCs mounted by MariaDB. |  |  |
| `metrics` _[MariadbMetrics](#mariadbmetrics)_ | Metrics configures metrics and how to scrape them. |  |  |
| `tls` _[TLS](#tls)_ | TLS defines the PKI to be used with MariaDB. |  |  |
| `replication` _[Replication](#replication)_ | Replication configures high availability via replication. This feature is still in alpha, use Galera if you are looking for a more production-ready HA. |  |  |
| `galera` _[Galera](#galera)_ | Replication configures high availability via Galera. |  |  |
| `maxScaleRef` _[ObjectReference](#objectreference)_ | MaxScaleRef is a reference to a MaxScale resource to be used with the current MariaDB.<br />Providing this field implies delegating high availability tasks such as primary failover to MaxScale. |  |  |
| `maxScale` _[MariaDBMaxScaleSpec](#mariadbmaxscalespec)_ | MaxScale is the MaxScale specification that defines the MaxScale resource to be used with the current MariaDB.<br />When enabling this field, MaxScaleRef is automatically set. |  |  |
| `replicas` _integer_ | Replicas indicates the number of desired instances. | 1 |  |
| `replicasAllowEvenNumber` _boolean_ | disables the validation check for an odd number of replicas. | false |  |
| `port` _integer_ | Port where the instances will be listening for connections. | 3306 |  |
| `servicePorts` _[ServicePort](#serviceport) array_ | ServicePorts is the list of additional named ports to be added to the Services created by the operator. |  |  |
| `podDisruptionBudget` _[PodDisruptionBudget](#poddisruptionbudget)_ | PodDisruptionBudget defines the budget for replica availability. |  |  |
| `updateStrategy` _[UpdateStrategy](#updatestrategy)_ | UpdateStrategy defines how a MariaDB resource is updated. |  |  |
| `service` _[ServiceTemplate](#servicetemplate)_ | Service defines a template to configure the general Service object.<br />The network traffic of this Service will be routed to all Pods. |  |  |
| `connection` _[ConnectionTemplate](#connectiontemplate)_ | Connection defines a template to configure the general Connection object.<br />This Connection provides the initial User access to the initial Database.<br />It will make use of the Service to route network traffic to all Pods. |  |  |
| `primaryService` _[ServiceTemplate](#servicetemplate)_ | PrimaryService defines a template to configure the primary Service object.<br />The network traffic of this Service will be routed to the primary Pod. |  |  |
| `primaryConnection` _[ConnectionTemplate](#connectiontemplate)_ | PrimaryConnection defines a template to configure the primary Connection object.<br />This Connection provides the initial User access to the initial Database.<br />It will make use of the PrimaryService to route network traffic to the primary Pod. |  |  |
| `secondaryService` _[ServiceTemplate](#servicetemplate)_ | SecondaryService defines a template to configure the secondary Service object.<br />The network traffic of this Service will be routed to the secondary Pods. |  |  |
| `secondaryConnection` _[ConnectionTemplate](#connectiontemplate)_ | SecondaryConnection defines a template to configure the secondary Connection object.<br />This Connection provides the initial User access to the initial Database.<br />It will make use of the SecondaryService to route network traffic to the secondary Pods. |  |  |


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
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
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
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | Interval defines the config synchronization interval. It is defaulted if not provided. |  |  |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | Interval defines the config synchronization timeout. It is defaulted if not provided. |  |  |


#### MaxScaleListener



MaxScaleListener defines how the MaxScale server will listen for connections.



_Appears in:_
- [MaxScaleService](#maxscaleservice)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not.<br />This can be useful for maintenance, as disabling the reconciliation prevents the operator from interfering with user operations during maintenance activities. | false |  |
| `name` _string_ | Name is the identifier of the listener. It is defaulted if not provided |  |  |
| `port` _integer_ | Port is the network port where the MaxScale server will listen. |  | Required: \{\} <br /> |
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
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not.<br />This can be useful for maintenance, as disabling the reconciliation prevents the operator from interfering with user operations during maintenance activities. | false |  |
| `name` _string_ | Name is the identifier of the monitor. It is defaulted if not provided. |  |  |
| `module` _[MonitorModule](#monitormodule)_ | Module is the module to use to monitor MariaDB servers. It is mandatory when no MariaDB reference is provided. |  |  |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | Interval used to monitor MariaDB servers. It is defaulted if not provided. |  |  |
| `cooperativeMonitoring` _[CooperativeMonitoring](#cooperativemonitoring)_ | CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors. It is defaulted when HA is enabled. |  | Enum: [majority_of_all majority_of_running] <br /> |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the monitor.<br />Any parameter supported by MaxScale may be specified here. See reference:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-common-monitor-parameters/.<br />Monitor specific parameter are also suported:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-galera-monitor/#galera-monitor-optional-parameters.<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-monitor/#configuration. |  |  |


#### MaxScalePodTemplate



MaxScalePodTemplate defines a template for MaxScale Pods.



_Appears in:_
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](#localobjectreference) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](#podsecuritycontext)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](#topologyspreadconstraint) array_ | TopologySpreadConstraints to be used in the Pod. |  |  |


#### MaxScaleServer



MaxScaleServer defines a MariaDB server to forward traffic to.



_Appears in:_
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the identifier of the MariaDB server. |  | Required: \{\} <br /> |
| `address` _string_ | Address is the network address of the MariaDB server. |  | Required: \{\} <br /> |
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
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not.<br />This can be useful for maintenance, as disabling the reconciliation prevents the operator from interfering with user operations during maintenance activities. | false |  |
| `name` _string_ | Name is the identifier of the MaxScale service. |  | Required: \{\} <br /> |
| `router` _[ServiceRouter](#servicerouter)_ | Router is the type of router to use. |  | Enum: [readwritesplit readconnroute] <br />Required: \{\} <br /> |
| `listener` _[MaxScaleListener](#maxscalelistener)_ | MaxScaleListener defines how the MaxScale server will listen for connections. |  | Required: \{\} <br /> |
| `params` _object (keys:string, values:string)_ | Params defines extra parameters to pass to the service.<br />Any parameter supported by MaxScale may be specified here. See reference:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#service_1.<br />Router specific parameter are also suported:<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-readwritesplit/#configuration.<br />https://mariadb.com/kb/en/mariadb-maxscale-2308-readconnroute/#configuration. |  |  |


#### MaxScaleSpec



MaxScaleSpec defines the desired state of MaxScale.



_Appears in:_
- [MaxScale](#maxscale)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ | Command to be used in the Container. |  |  |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `env` _[EnvVar](#envvar) array_ | Env represents the environment variables to be injected in a container. |  |  |
| `envFrom` _[EnvFromSource](#envfromsource) array_ | EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container. |  |  |
| `volumeMounts` _[VolumeMount](#volumemount) array_ | VolumeMounts to be used in the Container. |  |  |
| `livenessProbe` _[Probe](#probe)_ | LivenessProbe to be used in the Container. |  |  |
| `readinessProbe` _[Probe](#probe)_ | ReadinessProbe to be used in the Container. |  |  |
| `startupProbe` _[Probe](#probe)_ | StartupProbe to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](#localobjectreference) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](#podsecuritycontext)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](#topologyspreadconstraint) array_ | TopologySpreadConstraints to be used in the Pod. |  |  |
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not.<br />This can be useful for maintenance, as disabling the reconciliation prevents the operator from interfering with user operations during maintenance activities. | false |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to the MariaDB that MaxScale points to. It is used to initialize the servers field. |  |  |
| `servers` _[MaxScaleServer](#maxscaleserver) array_ | Servers are the MariaDB servers to forward traffic to. It is required if 'spec.mariaDbRef' is not provided. |  |  |
| `image` _string_ | Image name to be used by the MaxScale instances. The supported format is `<image>:<tag>`.<br />Only MaxScale official images are supported. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`. |  | Enum: [Always Never IfNotPresent] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |
| `services` _[MaxScaleService](#maxscaleservice) array_ | Services define how the traffic is forwarded to the MariaDB servers. It is defaulted if not provided. |  |  |
| `monitor` _[MaxScaleMonitor](#maxscalemonitor)_ | Monitor monitors MariaDB server instances. It is required if 'spec.mariaDbRef' is not provided. |  |  |
| `admin` _[MaxScaleAdmin](#maxscaleadmin)_ | Admin configures the admin REST API and GUI. |  |  |
| `config` _[MaxScaleConfig](#maxscaleconfig)_ | Config defines the MaxScale configuration. |  |  |
| `auth` _[MaxScaleAuth](#maxscaleauth)_ | Auth defines the credentials required for MaxScale to connect to MariaDB. |  |  |
| `metrics` _[MaxScaleMetrics](#maxscalemetrics)_ | Metrics configures metrics and how to scrape them. |  |  |
| `tls` _[MaxScaleTLS](#maxscaletls)_ | TLS defines the PKI to be used with MaxScale. |  |  |
| `connection` _[ConnectionTemplate](#connectiontemplate)_ | Connection provides a template to define the Connection for MaxScale. |  |  |
| `replicas` _integer_ | Replicas indicates the number of desired instances. | 1 |  |
| `podDisruptionBudget` _[PodDisruptionBudget](#poddisruptionbudget)_ | PodDisruptionBudget defines the budget for replica availability. |  |  |
| `updateStrategy` _[StatefulSetUpdateStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#statefulsetupdatestrategy-v1-apps)_ | UpdateStrategy defines the update strategy for the StatefulSet object. |  |  |
| `kubernetesService` _[ServiceTemplate](#servicetemplate)_ | KubernetesService defines a template for a Kubernetes Service object to connect to MaxScale. |  |  |
| `guiKubernetesService` _[ServiceTemplate](#servicetemplate)_ | GuiKubernetesService defines a template for a Kubernetes Service object to connect to MaxScale's GUI. |  |  |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. If not defined, it defaults to 10s. |  |  |


#### MaxScaleTLS



TLS defines the PKI to be used with MaxScale.



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled indicates whether TLS is enabled, determining if certificates should be issued and mounted to the MaxScale instance.<br />It is enabled by default when the referred MariaDB instance (via mariaDbRef) has TLS enabled and enforced. |  |  |
| `adminCASecretRef` _[LocalObjectReference](#localobjectreference)_ | AdminCASecretRef is a reference to a Secret containing the admin certificate authority keypair. It is used to establish trust and issue certificates for the MaxScale's administrative REST API and GUI.<br />One of:<br />- Secret containing both the 'ca.crt' and 'ca.key' keys. This allows you to bring your own CA to Kubernetes to issue certificates.<br />- Secret containing only the 'ca.crt' in order to establish trust. In this case, either adminCertSecretRef or adminCertIssuerRef fields must be provided.<br />If not provided, a self-signed CA will be provisioned to issue the server certificate. |  |  |
| `adminCertSecretRef` _[LocalObjectReference](#localobjectreference)_ | AdminCertSecretRef is a reference to a TLS Secret used by the MaxScale's administrative REST API and GUI. |  |  |
| `adminCertIssuerRef` _[ObjectReference](#objectreference)_ | AdminCertIssuerRef is a reference to a cert-manager issuer object used to issue the MaxScale's administrative REST API and GUI certificate. cert-manager must be installed previously in the cluster.<br />It is mutually exclusive with adminCertSecretRef.<br />By default, the Secret field 'ca.crt' provisioned by cert-manager will be added to the trust chain. A custom trust bundle may be specified via adminCASecretRef. |  |  |
| `listenerCASecretRef` _[LocalObjectReference](#localobjectreference)_ | ListenerCASecretRef is a reference to a Secret containing the listener certificate authority keypair. It is used to establish trust and issue certificates for the MaxScale's listeners.<br />One of:<br />- Secret containing both the 'ca.crt' and 'ca.key' keys. This allows you to bring your own CA to Kubernetes to issue certificates.<br />- Secret containing only the 'ca.crt' in order to establish trust. In this case, either listenerCertSecretRef or listenerCertIssuerRef fields must be provided.<br />If not provided, a self-signed CA will be provisioned to issue the listener certificate. |  |  |
| `listenerCertSecretRef` _[LocalObjectReference](#localobjectreference)_ | ListenerCertSecretRef is a reference to a TLS Secret used by the MaxScale's listeners. |  |  |
| `listenerCertIssuerRef` _[ObjectReference](#objectreference)_ | ListenerCertIssuerRef is a reference to a cert-manager issuer object used to issue the MaxScale's listeners certificate. cert-manager must be installed previously in the cluster.<br />It is mutually exclusive with listenerCertSecretRef.<br />By default, the Secret field 'ca.crt' provisioned by cert-manager will be added to the trust chain. A custom trust bundle may be specified via listenerCASecretRef. |  |  |
| `serverCASecretRef` _[LocalObjectReference](#localobjectreference)_ | ServerCASecretRef is a reference to a Secret containing the MariaDB server CA certificates. It is used to establish trust with MariaDB servers.<br />The Secret should contain a 'ca.crt' key in order to establish trust.<br />If not provided, and the reference to a MariaDB resource is set (mariaDbRef), it will be defaulted to the referred MariaDB CA bundle. |  |  |
| `serverCertSecretRef` _[LocalObjectReference](#localobjectreference)_ | ServerCertSecretRef is a reference to a TLS Secret used by MaxScale to connect to the MariaDB servers.<br />If not provided, and the reference to a MariaDB resource is set (mariaDbRef), it will be defaulted to the referred MariaDB client certificate (clientCertSecretRef). |  |  |
| `verifyPeerCertificate` _boolean_ | VerifyPeerCertificate specifies whether the peer certificate's signature should be validated against the CA.<br />It is disabled by default. |  |  |
| `verifyPeerHost` _boolean_ | VerifyPeerHost specifies whether the peer certificate's SANs should match the peer host.<br />It is disabled by default. |  |  |
| `replicationSSLEnabled` _boolean_ | ReplicationSSLEnabled specifies whether the replication SSL is enabled. If enabled, the SSL options will be added to the server configuration.<br />It is enabled by default when the referred MariaDB instance (via mariaDbRef) has replication enabled.<br />If the MariaDB servers are manually provided by the user via the 'servers' field, this must be set by the user as well. |  |  |


#### Metadata



Metadata defines the metadata to added to resources.



_Appears in:_
- [BackupSpec](#backupspec)
- [Exporter](#exporter)
- [GaleraInitJob](#galerainitjob)
- [GaleraRecoveryJob](#galerarecoveryjob)
- [Job](#job)
- [JobPodTemplate](#jobpodtemplate)
- [MariaDBSpec](#mariadbspec)
- [MaxScalePodTemplate](#maxscalepodtemplate)
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

| Field | Description |
| --- | --- |
| `mariadbmon` | MonitorModuleMariadb is a monitor to be used with MariaDB servers.<br /> |
| `galeramon` | MonitorModuleGalera is a monitor to be used with Galera servers.<br /> |


#### NFSVolumeSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nfsvolumesource-v1-core.



_Appears in:_
- [StorageVolumeSource](#storagevolumesource)
- [Volume](#volume)
- [VolumeSource](#volumesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `server` _string_ |  |  |  |
| `path` _string_ |  |  |  |
| `readOnly` _boolean_ |  |  |  |


#### NodeAffinity



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeaffinity-v1-core



_Appears in:_
- [Affinity](#affinity)
- [AffinityConfig](#affinityconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requiredDuringSchedulingIgnoredDuringExecution` _[NodeSelector](#nodeselector)_ |  |  |  |
| `preferredDuringSchedulingIgnoredDuringExecution` _[PreferredSchedulingTerm](#preferredschedulingterm) array_ |  |  |  |


#### NodeSelector



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeselector-v1-core



_Appears in:_
- [NodeAffinity](#nodeaffinity)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodeSelectorTerms` _[NodeSelectorTerm](#nodeselectorterm) array_ |  |  |  |




#### NodeSelectorTerm

_Underlying type:_ _[struct{MatchExpressions []NodeSelectorRequirement "json:\"matchExpressions,omitempty\""; MatchFields []NodeSelectorRequirement "json:\"matchFields,omitempty\""}](#struct{matchexpressions-[]nodeselectorrequirement-"json:\"matchexpressions,omitempty\"";-matchfields-[]nodeselectorrequirement-"json:\"matchfields,omitempty\""})_

Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeselectorterm-v1-core



_Appears in:_
- [NodeSelector](#nodeselector)
- [PreferredSchedulingTerm](#preferredschedulingterm)



#### ObjectFieldSelector



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectfieldselector-v1-core.



_Appears in:_
- [EnvVarSource](#envvarsource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ |  |  |  |
| `fieldPath` _string_ |  |  |  |


#### ObjectReference



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectreference-v1-core.



_Appears in:_
- [ConnectionSpec](#connectionspec)
- [MariaDBRef](#mariadbref)
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `namespace` _string_ |  |  |  |


#### PasswordPlugin



PasswordPlugin defines the password plugin and its arguments.



_Appears in:_
- [MariaDBSpec](#mariadbspec)
- [UserSpec](#userspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pluginNameSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | PluginNameSecretKeyRef is a reference to the authentication plugin to be used by the User.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the authentication plugin. |  |  |
| `pluginArgSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | PluginArgSecretKeyRef is a reference to the arguments to be provided to the authentication plugin for the User.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the authentication plugin arguments. |  |  |


#### PersistentVolumeClaimSpec



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeclaimspec-v1-core.



_Appears in:_
- [BackupStagingStorage](#backupstagingstorage)
- [BackupStorage](#backupstorage)
- [VolumeClaimTemplate](#volumeclaimtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeaccessmode-v1-core) array_ |  |  |  |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)_ |  |  |  |
| `resources` _[VolumeResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumeresourcerequirements-v1-core)_ |  |  |  |
| `storageClassName` _string_ |  |  |  |


#### PersistentVolumeClaimVolumeSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeclaimvolumesource-v1-core.



_Appears in:_
- [StorageVolumeSource](#storagevolumesource)
- [Volume](#volume)
- [VolumeSource](#volumesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `claimName` _string_ |  |  |  |
| `readOnly` _boolean_ |  |  |  |


#### PodAffinityTerm



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podaffinityterm-v1-core.



_Appears in:_
- [PodAntiAffinity](#podantiaffinity)
- [WeightedPodAffinityTerm](#weightedpodaffinityterm)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labelSelector` _[LabelSelector](#labelselector)_ |  |  |  |
| `topologyKey` _string_ |  |  |  |


#### PodAntiAffinity



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podantiaffinity-v1-core.



_Appears in:_
- [Affinity](#affinity)
- [AffinityConfig](#affinityconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requiredDuringSchedulingIgnoredDuringExecution` _[PodAffinityTerm](#podaffinityterm) array_ |  |  |  |
| `preferredDuringSchedulingIgnoredDuringExecution` _[WeightedPodAffinityTerm](#weightedpodaffinityterm) array_ |  |  |  |


#### PodDisruptionBudget



PodDisruptionBudget is the Pod availability bundget for a MariaDB



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minAvailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ | MinAvailable defines the number of minimum available Pods. |  |  |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ | MaxUnavailable defines the number of maximum unavailable Pods. |  |  |


#### PodSecurityContext



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podsecuritycontext-v1-core



_Appears in:_
- [BackupSpec](#backupspec)
- [Exporter](#exporter)
- [JobPodTemplate](#jobpodtemplate)
- [MariaDBSpec](#mariadbspec)
- [MaxScalePodTemplate](#maxscalepodtemplate)
- [MaxScaleSpec](#maxscalespec)
- [PodTemplate](#podtemplate)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `seLinuxOptions` _[SELinuxOptions](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#selinuxoptions-v1-core)_ |  |  |  |
| `runAsUser` _integer_ |  |  |  |
| `runAsGroup` _integer_ |  |  |  |
| `runAsNonRoot` _boolean_ |  |  |  |
| `supplementalGroups` _integer array_ |  |  |  |
| `fsGroup` _integer_ |  |  |  |
| `fsGroupChangePolicy` _[PodFSGroupChangePolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podfsgroupchangepolicy-v1-core)_ |  |  |  |
| `seccompProfile` _[SeccompProfile](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#seccompprofile-v1-core)_ |  |  |  |
| `appArmorProfile` _[AppArmorProfile](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#apparmorprofile-v1-core)_ |  |  |  |


#### PodTemplate



PodTemplate defines a template to configure Container objects.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](#localobjectreference) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `initContainers` _[Container](#container) array_ | InitContainers to be used in the Pod. |  |  |
| `sidecarContainers` _[Container](#container) array_ | SidecarContainers to be used in the Pod. |  |  |
| `podSecurityContext` _[PodSecurityContext](#podsecuritycontext)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `volumes` _[Volume](#volume) array_ | Volumes to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](#topologyspreadconstraint) array_ | TopologySpreadConstraints to be used in the Pod. |  |  |


#### PreferredSchedulingTerm



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#preferredschedulingterm-v1-core



_Appears in:_
- [NodeAffinity](#nodeaffinity)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `weight` _integer_ |  |  |  |
| `preference` _[NodeSelectorTerm](#nodeselectorterm)_ |  |  |  |


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


#### Probe



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core.



_Appears in:_
- [ContainerTemplate](#containertemplate)
- [GaleraAgent](#galeraagent)
- [GaleraInit](#galerainit)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `exec` _[ExecAction](#execaction)_ |  |  |  |
| `httpGet` _[HTTPGetAction](#httpgetaction)_ |  |  |  |
| `tcpSocket` _[TCPSocketAction](#tcpsocketaction)_ |  |  |  |
| `initialDelaySeconds` _integer_ |  |  |  |
| `timeoutSeconds` _integer_ |  |  |  |
| `periodSeconds` _integer_ |  |  |  |
| `successThreshold` _integer_ |  |  |  |
| `failureThreshold` _integer_ |  |  |  |


#### ProbeHandler



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core.



_Appears in:_
- [Probe](#probe)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `exec` _[ExecAction](#execaction)_ |  |  |  |
| `httpGet` _[HTTPGetAction](#httpgetaction)_ |  |  |  |
| `tcpSocket` _[TCPSocketAction](#tcpsocketaction)_ |  |  |  |


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
| `connectionTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | ConnectionTimeout to be used when the replica connects to the primary. |  |  |
| `connectionRetries` _integer_ | ConnectionRetries to be used when the replica connects to the primary. |  |  |
| `syncTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | SyncTimeout defines the timeout for a replica to be synced with the primary when performing a primary switchover.<br />If the timeout is reached, the replica GTID will be reset and the switchover will continue. |  |  |


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




#### ResourceRequirements



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core.



_Appears in:_
- [BackupSpec](#backupspec)
- [Container](#container)
- [ContainerTemplate](#containertemplate)
- [Exporter](#exporter)
- [GaleraAgent](#galeraagent)
- [GaleraInit](#galerainit)
- [GaleraInitJob](#galerainitjob)
- [GaleraRecoveryJob](#galerarecoveryjob)
- [Job](#job)
- [JobContainerTemplate](#jobcontainertemplate)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)



#### Restore



Restore is the Schema for the restores API. It is used to define restore jobs and its restoration source.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `Restore` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RestoreSpec](#restorespec)_ |  |  |  |


#### RestoreSource



RestoreSource defines a source for restoring a MariaDB.



_Appears in:_
- [BootstrapFrom](#bootstrapfrom)
- [RestoreSpec](#restorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backupRef` _[LocalObjectReference](#localobjectreference)_ | BackupRef is a reference to a Backup object. It has priority over S3 and Volume. |  |  |
| `s3` _[S3](#s3)_ | S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume. |  |  |
| `volume` _[StorageVolumeSource](#storagevolumesource)_ | Volume is a Kubernetes Volume object that contains a backup. |  |  |
| `targetRecoveryTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ | TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.<br />It is used to determine the closest restoration source in time. |  |  |
| `stagingStorage` _[BackupStagingStorage](#backupstagingstorage)_ | StagingStorage defines the temporary storage used to keep external backups (i.e. S3) while they are being processed.<br />It defaults to an emptyDir volume, meaning that the backups will be temporarily stored in the node where the Restore Job is scheduled. |  |  |


#### RestoreSpec



RestoreSpec defines the desired state of restore



_Appears in:_
- [Restore](#restore)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](#localobjectreference) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](#podsecuritycontext)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `backupRef` _[LocalObjectReference](#localobjectreference)_ | BackupRef is a reference to a Backup object. It has priority over S3 and Volume. |  |  |
| `s3` _[S3](#s3)_ | S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume. |  |  |
| `volume` _[StorageVolumeSource](#storagevolumesource)_ | Volume is a Kubernetes Volume object that contains a backup. |  |  |
| `targetRecoveryTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ | TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.<br />It is used to determine the closest restoration source in time. |  |  |
| `stagingStorage` _[BackupStagingStorage](#backupstagingstorage)_ | StagingStorage defines the temporary storage used to keep external backups (i.e. S3) while they are being processed.<br />It defaults to an emptyDir volume, meaning that the backups will be temporarily stored in the node where the Restore Job is scheduled. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: \{\} <br /> |
| `database` _string_ | Database defines the logical database to be restored. If not provided, all databases available in the backup are restored.<br />IMPORTANT: The database must previously exist. |  |  |
| `logLevel` _string_ | LogLevel to be used n the Backup Job. It defaults to 'info'. | info |  |
| `backoffLimit` _integer_ | BackoffLimit defines the maximum number of attempts to successfully perform a Backup. | 5 |  |
| `restartPolicy` _[RestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#restartpolicy-v1-core)_ | RestartPolicy to be added to the Backup Job. | OnFailure | Enum: [Always OnFailure Never] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |


#### S3







_Appears in:_
- [BackupStorage](#backupstorage)
- [BootstrapFrom](#bootstrapfrom)
- [RestoreSource](#restoresource)
- [RestoreSpec](#restorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucket` _string_ | Bucket is the name Name of the bucket to store backups. |  | Required: \{\} <br /> |
| `endpoint` _string_ | Endpoint is the S3 API endpoint without scheme. |  | Required: \{\} <br /> |
| `region` _string_ | Region is the S3 region name to use. |  |  |
| `prefix` _string_ | Prefix indicates a folder/subfolder in the bucket. For example: mariadb/ or mariadb/backups. A trailing slash '/' is added if not provided. |  |  |
| `accessKeyIdSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | AccessKeyIdSecretKeyRef is a reference to a Secret key containing the S3 access key id. |  |  |
| `secretAccessKeySecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | AccessKeyIdSecretKeyRef is a reference to a Secret key containing the S3 secret key. |  |  |
| `sessionTokenSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | SessionTokenSecretKeyRef is a reference to a Secret key containing the S3 session token. |  |  |
| `tls` _[TLSS3](#tlss3)_ | TLS provides the configuration required to establish TLS connections with S3. |  |  |


#### SQLTemplate



SQLTemplate defines a template to customize SQL objects.



_Appears in:_
- [DatabaseSpec](#databasespec)
- [GrantSpec](#grantspec)
- [UserSpec](#userspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |  |  |
| `cleanupPolicy` _[CleanupPolicy](#cleanuppolicy)_ | CleanupPolicy defines the behavior for cleaning up a SQL resource. |  | Enum: [Skip Delete] <br /> |


#### SST

_Underlying type:_ _string_

SST is the Snapshot State Transfer used when new Pods join the cluster.
More info: https://galeracluster.com/library/documentation/sst.html.



_Appears in:_
- [Galera](#galera)
- [GaleraSpec](#galeraspec)

| Field | Description |
| --- | --- |
| `rsync` | SSTRsync is an SST based on rsync.<br /> |
| `mariabackup` | SSTMariaBackup is an SST based on mariabackup. It is the recommended SST.<br /> |
| `mysqldump` | SSTMysqldump is an SST based on mysqldump.<br /> |


#### Schedule



Schedule contains parameters to define a schedule



_Appears in:_
- [BackupSpec](#backupspec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cron` _string_ | Cron is a cron expression that defines the schedule. |  | Required: \{\} <br /> |
| `suspend` _boolean_ | Suspend defines whether the schedule is active or not. | false |  |


#### SecretKeySelector



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core.



_Appears in:_
- [ConnectionSpec](#connectionspec)
- [EnvVarSource](#envvarsource)
- [GeneratedSecretKeyRef](#generatedsecretkeyref)
- [MariaDBSpec](#mariadbspec)
- [PasswordPlugin](#passwordplugin)
- [S3](#s3)
- [SqlJobSpec](#sqljobspec)
- [TLSS3](#tlss3)
- [UserSpec](#userspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `key` _string_ |  |  |  |


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


#### SecretVolumeSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretvolumesource-v1-core.



_Appears in:_
- [Volume](#volume)
- [VolumeSource](#volumesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretName` _string_ |  |  |  |
| `defaultMode` _integer_ |  |  |  |


#### SecurityContext



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core.



_Appears in:_
- [BackupSpec](#backupspec)
- [ContainerTemplate](#containertemplate)
- [Exporter](#exporter)
- [GaleraAgent](#galeraagent)
- [GaleraInit](#galerainit)
- [JobContainerTemplate](#jobcontainertemplate)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)
- [RestoreSpec](#restorespec)
- [SqlJobSpec](#sqljobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `capabilities` _[Capabilities](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#capabilities-v1-core)_ |  |  |  |
| `privileged` _boolean_ |  |  |  |
| `runAsUser` _integer_ |  |  |  |
| `runAsGroup` _integer_ |  |  |  |
| `runAsNonRoot` _boolean_ |  |  |  |
| `readOnlyRootFilesystem` _boolean_ |  |  |  |
| `allowPrivilegeEscalation` _boolean_ |  |  |  |


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


#### ServicePort



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `port` _integer_ |  |  |  |


#### ServiceRouter

_Underlying type:_ _string_

ServiceRouter defines the type of service router.



_Appears in:_
- [MaxScaleService](#maxscaleservice)

| Field | Description |
| --- | --- |
| `readwritesplit` | ServiceRouterReadWriteSplit splits the load based on the queries. Write queries are performed on master and read queries on the replicas.<br /> |
| `readconnroute` | ServiceRouterReadConnRoute splits the load based on the connections. Each connection is assigned to a server.<br /> |


#### ServiceTemplate



ServiceTemplate defines a template to customize Service objects.



_Appears in:_
- [MariaDBMaxScaleSpec](#mariadbmaxscalespec)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#servicetype-v1-core)_ | Type is the Service type. One of `ClusterIP`, `NodePort` or `LoadBalancer`. If not defined, it defaults to `ClusterIP`. | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br /> |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `loadBalancerIP` _string_ | LoadBalancerIP Service field. |  |  |
| `loadBalancerSourceRanges` _string array_ | LoadBalancerSourceRanges Service field. |  |  |
| `externalTrafficPolicy` _[ServiceExternalTrafficPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceexternaltrafficpolicy-v1-core)_ | ExternalTrafficPolicy Service field. |  |  |
| `sessionAffinity` _[ServiceAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceaffinity-v1-core)_ | SessionAffinity Service field. |  |  |
| `allocateLoadBalancerNodePorts` _boolean_ | AllocateLoadBalancerNodePorts Service field. |  |  |


#### SqlJob



SqlJob is the Schema for the sqljobs API. It is used to run sql scripts as jobs.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `SqlJob` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[SqlJobSpec](#sqljobspec)_ |  |  |  |


#### SqlJobSpec



SqlJobSpec defines the desired state of SqlJob



_Appears in:_
- [SqlJob](#sqljob)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `args` _string array_ | Args to be used in the Container. |  |  |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resouces describes the compute resource requirements. |  |  |
| `securityContext` _[SecurityContext](#securitycontext)_ | SecurityContext holds security configuration that will be applied to a container. |  |  |
| `podMetadata` _[Metadata](#metadata)_ | PodMetadata defines extra metadata for the Pod. |  |  |
| `imagePullSecrets` _[LocalObjectReference](#localobjectreference) array_ | ImagePullSecrets is the list of pull Secrets to be used to pull the image. |  |  |
| `podSecurityContext` _[PodSecurityContext](#podsecuritycontext)_ | SecurityContext holds pod-level security attributes and common container settings. |  |  |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to be used by the Pods. |  |  |
| `affinity` _[AffinityConfig](#affinityconfig)_ | Affinity to be used in the Pod. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be used in the Pod. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ | Tolerations to be used in the Pod. |  |  |
| `priorityClassName` _string_ | PriorityClassName to be used in the Pod. |  |  |
| `successfulJobsHistoryLimit` _integer_ | SuccessfulJobsHistoryLimit defines the maximum number of successful Jobs to be displayed. |  | Minimum: 0 <br /> |
| `failedJobsHistoryLimit` _integer_ | FailedJobsHistoryLimit defines the maximum number of failed Jobs to be displayed. |  | Minimum: 0 <br /> |
| `timeZone` _string_ | TimeZone defines the timezone associated with the cron expression. |  |  |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: \{\} <br /> |
| `schedule` _[Schedule](#schedule)_ | Schedule defines when the SqlJob will be executed. |  |  |
| `username` _string_ | Username to be impersonated when executing the SqlJob. |  | Required: \{\} <br /> |
| `passwordSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | UserPasswordSecretKeyRef is a reference to the impersonated user's password to be used when executing the SqlJob. |  | Required: \{\} <br /> |
| `tlsCASecretRef` _[LocalObjectReference](#localobjectreference)_ | TLSCACertSecretRef is a reference toa CA Secret used to establish trust when executing the SqlJob.<br />If not provided, the CA bundle provided by the referred MariaDB is used. |  |  |
| `tlsClientCertSecretRef` _[LocalObjectReference](#localobjectreference)_ | TLSClientCertSecretRef is a reference to a Kubernetes TLS Secret used as authentication when executing the SqlJob.<br />If not provided, the client certificate provided by the referred MariaDB is used. |  |  |
| `database` _string_ | Username to be used when executing the SqlJob. |  |  |
| `dependsOn` _[LocalObjectReference](#localobjectreference) array_ | DependsOn defines dependencies with other SqlJob objectecs. |  |  |
| `sql` _string_ | Sql is the script to be executed by the SqlJob. |  |  |
| `sqlConfigMapKeyRef` _[ConfigMapKeySelector](#configmapkeyselector)_ | SqlConfigMapKeyRef is a reference to a ConfigMap containing the Sql script.<br />It is defaulted to a ConfigMap with the contents of the Sql field. |  |  |
| `backoffLimit` _integer_ | BackoffLimit defines the maximum number of attempts to successfully execute a SqlJob. | 5 |  |
| `restartPolicy` _[RestartPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#restartpolicy-v1-core)_ | RestartPolicy to be added to the SqlJob Pod. | OnFailure | Enum: [Always OnFailure Never] <br /> |
| `inheritMetadata` _[Metadata](#metadata)_ | InheritMetadata defines the metadata to be inherited by children resources. |  |  |


#### Storage



Storage defines the storage options to be used for provisioning the PVCs mounted by MariaDB.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ephemeral` _boolean_ | Ephemeral indicates whether to use ephemeral storage in the PVCs. It is only compatible with non HA MariaDBs. |  |  |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#quantity-resource-api)_ | Size of the PVCs to be mounted by MariaDB. Required if not provided in 'VolumeClaimTemplate'. It superseeds the storage size specified in 'VolumeClaimTemplate'. |  |  |
| `storageClassName` _string_ | StorageClassName to be used to provision the PVCS. It superseeds the 'StorageClassName' specified in 'VolumeClaimTemplate'.<br />If not provided, the default 'StorageClass' configured in the cluster is used. |  |  |
| `resizeInUseVolumes` _boolean_ | ResizeInUseVolumes indicates whether the PVCs can be resized. The 'StorageClassName' used should have 'allowVolumeExpansion' set to 'true' to allow resizing.<br />It defaults to true. |  |  |
| `waitForVolumeResize` _boolean_ | WaitForVolumeResize indicates whether to wait for the PVCs to be resized before marking the MariaDB object as ready. This will block other operations such as cluster recovery while the resize is in progress.<br />It defaults to true. |  |  |
| `volumeClaimTemplate` _[VolumeClaimTemplate](#volumeclaimtemplate)_ | VolumeClaimTemplate provides a template to define the PVCs. |  |  |


#### StorageVolumeSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core.



_Appears in:_
- [BackupStagingStorage](#backupstagingstorage)
- [BackupStorage](#backupstorage)
- [BootstrapFrom](#bootstrapfrom)
- [RestoreSource](#restoresource)
- [RestoreSpec](#restorespec)
- [Volume](#volume)
- [VolumeSource](#volumesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `emptyDir` _[EmptyDirVolumeSource](#emptydirvolumesource)_ |  |  |  |
| `nfs` _[NFSVolumeSource](#nfsvolumesource)_ |  |  |  |
| `csi` _[CSIVolumeSource](#csivolumesource)_ |  |  |  |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](#persistentvolumeclaimvolumesource)_ |  |  |  |


#### SuspendTemplate



SuspendTemplate indicates whether the current resource should be suspended or not.



_Appears in:_
- [MariaDBSpec](#mariadbspec)
- [MaxScaleListener](#maxscalelistener)
- [MaxScaleMonitor](#maxscalemonitor)
- [MaxScaleService](#maxscaleservice)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `suspend` _boolean_ | Suspend indicates whether the current resource should be suspended or not.<br />This can be useful for maintenance, as disabling the reconciliation prevents the operator from interfering with user operations during maintenance activities. | false |  |


#### TCPSocketAction



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#tcpsocketaction-v1-core.



_Appears in:_
- [Probe](#probe)
- [ProbeHandler](#probehandler)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `port` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ |  |  |  |
| `host` _string_ |  |  |  |


#### TLS



TLS defines the PKI to be used with MariaDB.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled indicates whether TLS is enabled, determining if certificates should be issued and mounted to the MariaDB instance.<br />It is enabled by default. |  |  |
| `required` _boolean_ | Required specifies whether TLS must be enforced for all connections.<br />User TLS requirements take precedence over this.<br />It disabled by default. |  |  |
| `serverCASecretRef` _[LocalObjectReference](#localobjectreference)_ | ServerCASecretRef is a reference to a Secret containing the server certificate authority keypair. It is used to establish trust and issue server certificates.<br />One of:<br />- Secret containing both the 'ca.crt' and 'ca.key' keys. This allows you to bring your own CA to Kubernetes to issue certificates.<br />- Secret containing only the 'ca.crt' in order to establish trust. In this case, either serverCertSecretRef or serverCertIssuerRef must be provided.<br />If not provided, a self-signed CA will be provisioned to issue the server certificate. |  |  |
| `serverCertSecretRef` _[LocalObjectReference](#localobjectreference)_ | ServerCertSecretRef is a reference to a TLS Secret containing the server certificate.<br />It is mutually exclusive with serverCertIssuerRef. |  |  |
| `serverCertIssuerRef` _[ObjectReference](#objectreference)_ | ServerCertIssuerRef is a reference to a cert-manager issuer object used to issue the server certificate. cert-manager must be installed previously in the cluster.<br />It is mutually exclusive with serverCertSecretRef.<br />By default, the Secret field 'ca.crt' provisioned by cert-manager will be added to the trust chain. A custom trust bundle may be specified via serverCASecretRef. |  |  |
| `clientCASecretRef` _[LocalObjectReference](#localobjectreference)_ | ClientCASecretRef is a reference to a Secret containing the client certificate authority keypair. It is used to establish trust and issue client certificates.<br />One of:<br />- Secret containing both the 'ca.crt' and 'ca.key' keys. This allows you to bring your own CA to Kubernetes to issue certificates.<br />- Secret containing only the 'ca.crt' in order to establish trust. In this case, either clientCertSecretRef or clientCertIssuerRef fields must be provided.<br />If not provided, a self-signed CA will be provisioned to issue the client certificate. |  |  |
| `clientCertSecretRef` _[LocalObjectReference](#localobjectreference)_ | ClientCertSecretRef is a reference to a TLS Secret containing the client certificate.<br />It is mutually exclusive with clientCertIssuerRef. |  |  |
| `clientCertIssuerRef` _[ObjectReference](#objectreference)_ | ClientCertIssuerRef is a reference to a cert-manager issuer object used to issue the client certificate. cert-manager must be installed previously in the cluster.<br />It is mutually exclusive with clientCertSecretRef.<br />By default, the Secret field 'ca.crt' provisioned by cert-manager will be added to the trust chain. A custom trust bundle may be specified via clientCASecretRef. |  |  |
| `galeraSSTEnabled` _boolean_ | GaleraSSTEnabled determines whether Galera SST connections should use TLS.<br />It disabled by default. |  |  |


#### TLSRequirements



TLSRequirements specifies TLS requirements for the user to connect. See: https://mariadb.com/kb/en/securing-connections-for-client-and-server/#requiring-tls.



_Appears in:_
- [UserSpec](#userspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ssl` _boolean_ | SSL indicates that the user must connect via TLS. |  |  |
| `x509` _boolean_ | X509 indicates that the user must provide a valid x509 certificate to connect. |  |  |
| `issuer` _string_ | Issuer indicates that the TLS certificate provided by the user must be issued by a specific issuer. |  |  |
| `subject` _string_ | Subject indicates that the TLS certificate provided by the user must have a specific subject. |  |  |


#### TLSS3







_Appears in:_
- [S3](#s3)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled is a flag to enable TLS. |  |  |
| `caSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | CASecretKeyRef is a reference to a Secret key containing a CA bundle in PEM format used to establish TLS connections with S3.<br />By default, the system trust chain will be used, but you can use this field to add more CAs to the bundle. |  |  |


#### TopologySpreadConstraint



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core.



_Appears in:_
- [MariaDBSpec](#mariadbspec)
- [MaxScalePodTemplate](#maxscalepodtemplate)
- [MaxScaleSpec](#maxscalespec)
- [PodTemplate](#podtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxSkew` _integer_ |  |  |  |
| `topologyKey` _string_ |  |  |  |
| `whenUnsatisfiable` _[UnsatisfiableConstraintAction](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#unsatisfiableconstraintaction-v1-core)_ |  |  |  |
| `labelSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)_ |  |  |  |
| `minDomains` _integer_ |  |  |  |
| `nodeAffinityPolicy` _[NodeInclusionPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeinclusionpolicy-v1-core)_ |  |  |  |
| `nodeTaintsPolicy` _[NodeInclusionPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeinclusionpolicy-v1-core)_ |  |  |  |
| `matchLabelKeys` _string array_ |  |  |  |


#### UpdateStrategy



UpdateStrategy defines how a MariaDB resource is updated.



_Appears in:_
- [MariaDBSpec](#mariadbspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[UpdateType](#updatetype)_ | Type defines the type of updates. One of `ReplicasFirstPrimaryLast`, `RollingUpdate` or `OnDelete`. If not defined, it defaults to `ReplicasFirstPrimaryLast`. | ReplicasFirstPrimaryLast | Enum: [ReplicasFirstPrimaryLast RollingUpdate OnDelete Never] <br /> |
| `rollingUpdate` _[RollingUpdateStatefulSetStrategy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#rollingupdatestatefulsetstrategy-v1-apps)_ | RollingUpdate defines parameters for the RollingUpdate type. |  |  |
| `autoUpdateDataPlane` _boolean_ | AutoUpdateDataPlane indicates whether the Galera data-plane version (agent and init containers) should be automatically updated based on the operator version. It defaults to false.<br />Updating the operator will trigger updates on all the MariaDB instances that have this flag set to true. Thus, it is recommended to progressively set this flag after having updated the operator. |  |  |


#### UpdateType

_Underlying type:_ _string_

UpdateType defines the type of update for a MariaDB resource.



_Appears in:_
- [UpdateStrategy](#updatestrategy)

| Field | Description |
| --- | --- |
| `ReplicasFirstPrimaryLast` | ReplicasFirstPrimaryLastUpdateType indicates that the update will be applied to all replica Pods first and later on to the primary Pod.<br />The updates are applied one by one waiting until each Pod passes the readiness probe<br />i.e. the Pod gets synced and it is ready to receive traffic.<br /> |
| `RollingUpdate` | RollingUpdateUpdateType indicates that the update will be applied by the StatefulSet controller using the RollingUpdate strategy.<br />This strategy is unaware of the roles that the Pod have (primary or replica) and it will<br />perform the update following the StatefulSet ordinal, from higher to lower.<br /> |
| `OnDelete` | OnDeleteUpdateType indicates that the update will be applied by the StatefulSet controller using the OnDelete strategy.<br />The update will be done when the Pods get manually deleted by the user.<br /> |
| `Never` | NeverUpdateType indicates that the StatefulSet will never be updated.<br />This can be used to roll out updates progressively to a fleet of instances.<br /> |


#### User



User is the Schema for the users API.  It is used to define grants as if you were running a 'CREATE USER' statement.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `k8s.mariadb.com/v1alpha1` | | |
| `kind` _string_ | `User` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[UserSpec](#userspec)_ |  |  |  |


#### UserSpec



UserSpec defines the desired state of User



_Appears in:_
- [User](#user)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requeueInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RequeueInterval is used to perform requeue reconciliations. |  |  |
| `retryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta)_ | RetryInterval is the interval used to perform retries. |  |  |
| `cleanupPolicy` _[CleanupPolicy](#cleanuppolicy)_ | CleanupPolicy defines the behavior for cleaning up a SQL resource. |  | Enum: [Skip Delete] <br /> |
| `mariaDbRef` _[MariaDBRef](#mariadbref)_ | MariaDBRef is a reference to a MariaDB object. |  | Required: \{\} <br /> |
| `passwordSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | PasswordSecretKeyRef is a reference to the password to be used by the User.<br />If not provided, the account will be locked and the password will expire.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password. |  |  |
| `passwordHashSecretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | PasswordHashSecretKeyRef is a reference to the password hash to be used by the User.<br />If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password hash. |  |  |
| `passwordPlugin` _[PasswordPlugin](#passwordplugin)_ | PasswordPlugin is a reference to the password plugin and arguments to be used by the User. |  |  |
| `require` _[TLSRequirements](#tlsrequirements)_ | Require specifies TLS requirements for the user to connect. See: https://mariadb.com/kb/en/securing-connections-for-client-and-server/#requiring-tls. |  |  |
| `maxUserConnections` _integer_ | MaxUserConnections defines the maximum number of simultaneous connections that the User can establish. | 10 |  |
| `name` _string_ | Name overrides the default name provided by metadata.name. |  | MaxLength: 80 <br /> |
| `host` _string_ | Host related to the User. |  | MaxLength: 255 <br /> |


#### Volume



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core.



_Appears in:_
- [MariaDBSpec](#mariadbspec)
- [PodTemplate](#podtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `emptyDir` _[EmptyDirVolumeSource](#emptydirvolumesource)_ |  |  |  |
| `nfs` _[NFSVolumeSource](#nfsvolumesource)_ |  |  |  |
| `csi` _[CSIVolumeSource](#csivolumesource)_ |  |  |  |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](#persistentvolumeclaimvolumesource)_ |  |  |  |
| `secret` _[SecretVolumeSource](#secretvolumesource)_ |  |  |  |
| `configMap` _[ConfigMapVolumeSource](#configmapvolumesource)_ |  |  |  |


#### VolumeClaimTemplate



VolumeClaimTemplate defines a template to customize PVC objects.



_Appears in:_
- [GaleraConfig](#galeraconfig)
- [MaxScaleConfig](#maxscaleconfig)
- [Storage](#storage)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeaccessmode-v1-core) array_ |  |  |  |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)_ |  |  |  |
| `resources` _[VolumeResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumeresourcerequirements-v1-core)_ |  |  |  |
| `storageClassName` _string_ |  |  |  |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |


#### VolumeMount



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core.



_Appears in:_
- [Container](#container)
- [ContainerTemplate](#containertemplate)
- [GaleraAgent](#galeraagent)
- [GaleraInit](#galerainit)
- [MariaDBSpec](#mariadbspec)
- [MaxScaleSpec](#maxscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | This must match the Name of a Volume. |  |  |
| `readOnly` _boolean_ |  |  |  |
| `mountPath` _string_ |  |  |  |
| `subPath` _string_ |  |  |  |


#### VolumeSource



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core.



_Appears in:_
- [Volume](#volume)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `emptyDir` _[EmptyDirVolumeSource](#emptydirvolumesource)_ |  |  |  |
| `nfs` _[NFSVolumeSource](#nfsvolumesource)_ |  |  |  |
| `csi` _[CSIVolumeSource](#csivolumesource)_ |  |  |  |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](#persistentvolumeclaimvolumesource)_ |  |  |  |
| `secret` _[SecretVolumeSource](#secretvolumesource)_ |  |  |  |
| `configMap` _[ConfigMapVolumeSource](#configmapvolumesource)_ |  |  |  |


#### WaitPoint

_Underlying type:_ _string_

WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine.
More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.



_Appears in:_
- [ReplicaReplication](#replicareplication)

| Field | Description |
| --- | --- |
| `AfterSync` | WaitPointAfterSync indicates that the primary waits for the replica ACK before committing the transaction to the storage engine.<br />This is the default WaitPoint. It trades off performance for consistency.<br /> |
| `AfterCommit` | WaitPointAfterCommit indicates that the primary commits the transaction to the storage engine and waits for the replica ACK afterwards.<br />It trades off consistency for performance.<br /> |


#### WeightedPodAffinityTerm



Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#weightedpodaffinityterm-v1-core.



_Appears in:_
- [PodAntiAffinity](#podantiaffinity)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `weight` _integer_ |  |  |  |
| `podAffinityTerm` _[PodAffinityTerm](#podaffinityterm)_ |  |  |  |


