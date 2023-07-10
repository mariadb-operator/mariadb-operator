package v1alpha1

const (
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
)
