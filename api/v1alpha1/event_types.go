package v1alpha1

const (
	// ReasonReplicationResetStaleSwitchover indicates a stale switchover has been reset.
	ReasonReplicationResetStaleSwitchover = "ResetStaleSwitchover"
	// ReasonReplicationPrimaryLock indicates that primary tables have a read lock.
	ReasonReplicationPrimaryLock = "PrimaryLock"
	// ReasonReplicationPrimaryReadonly indicates that primary is being changed to readonly mode.
	ReasonReplicationPrimaryReadonly = "PrimaryReadonly"
	// ReasonReplicationReplicaSync indicates that replicas are being synced with primary.
	ReasonReplicationReplicaSync = "ReplicaSync"
	// ReasonReplicationReplicaSyncErr indicates that an error has happened while replicas were being synced with primary.
	ReasonReplicationReplicaSyncErr = "ReplicaSyncErr"
	// ReasonReplicationPrimaryNew indicates that a new primary is being configured.
	ReasonReplicationPrimaryNew = "PrimaryNew"
	// ReasonReplicationReplicaConn indicates that replicas are connecting to the new primary.
	ReasonReplicationReplicaConn = "ReplicaConn"
	// ReasonReplicationPrimaryToReplica indicates that current primary is being unlocked to become a replica.
	ReasonReplicationPrimaryToReplica = "PrimaryToReplica"

	// ReasonGaleraClusterHealthy indicates that the cluster is healthy,
	ReasonGaleraClusterHealthy = "GaleraClusterHealthy"
	// ReasonGaleraClusterNotHealthy indicates that the cluster is not healthy.
	ReasonGaleraClusterNotHealthy = "GaleraClusterNotHealthy"
	// ReasonGaleraClusterBootstrap indicates that the cluster is being bootstrapped.
	ReasonGaleraClusterBootstrap = "GaleraClusterBootstrap"
	// ReasonGaleraClusterBootstrapTimeout indicates that the cluster bootstrap has timed out.
	ReasonGaleraClusterBootstrapTimeout = "GaleraClusterBootstrapTimeout"
	// ReasonGaleraPodStateFetched indicates that the Pod state has been fetched successfully.
	ReasonGaleraPodStateFetched = "GaleraPodStateFetched"
	// ReasonGaleraPodRecovered indicates that the Pod has successfully recovered the sequence.
	ReasonGaleraPodRecovered = "GaleraPodRecovered"
	// ReasonGaleraPodSyncTimeout indicates that the Pod has timed out reaching the Sync state.
	ReasonGaleraPodSyncTimeout = "GaleraPodSyncTimeout"
	// ReasonGaleraPVCNotBound indicates that a Galera PVC is not in Bound phase, therefore the init process cannot be started.
	ReasonGaleraPVCNotBound = "GaleraPVCNotBound"

	// ReasonPrimarySwitching indicates that primary is being switched.
	ReasonPrimarySwitching = "PrimarySwitching"
	// ReasonPrimarySwitched indicates that primary has been switched.
	ReasonPrimarySwitched = "PrimarySwitched"

	// ReasonMaxScalePrimaryServerChanged indicates that the primary server managed by MaxScale has changed.
	ReasonMaxScalePrimaryServerChanged = "MaxScalePrimaryServerChanged"

	// ReasonWebhookUpdateFailed indicates that the webhook configuration update failed.
	ReasonWebhookUpdateFailed = "WebhookUpdateFailed"

	// ReasonCRDNotFound indicates that a third party CRD is not present in the cluster.
	ReasonCRDNotFound = "CRDNotFound"

	// SecretKeyNotFound indicates that a required Secret key could not be found.
	SecretKeyNotFound = "SecretKeyNotFound"

	// ReasonJobScheduled indicates that a Job has been scheduled.
	ReasonJobScheduled = "JobScheduled"

	// ReasonVolumeSnapshotCreated indicates that a VolumeSnapshot has been scheduled.
	ReasonVolumeSnapshotCreated = "VolumeSnapshotCreated"

	// ReasonMariaDBNotReady indicates that the MariaDB resource is not ready.
	ReasonMariaDBNotReady = "MariaDBNotReady"

	// ReasonMariaDBInitError indicates that an error has occurred during the MariaDB instance initialization.
	ReasonMariaDBInitError = "MariaDBInitError"

	// ReasonMariaDBScaleOutError indicates that an error has occurred during the MariaDB instance scale out operation.
	ReasonMariaDBScaleOutError = "MariaDBScaleOutError"
)
