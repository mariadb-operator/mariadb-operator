package v1alpha1

// Different event `actions`. This field is for sorting and filtering primarily and is for machines and automated processes.
// Ref: https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/event-v1/

const (
	// ActionReconciling indicates that the controller is reconciling a resource.
	ActionReconciling = "Reconciling"
	// ActionReconciling indicates that the archiver is archiving a binary log.
	ActionArchiving = "Archiving"
)
