package v1alpha1

const (
	ConditionTypeReady        string = "Ready"
	ConditionTypeBootstrapped string = "Bootstrapped"
	ConditionTypeComplete     string = "Complete"
)

const (
	ConditionReasonStatefulSetNotReady string = "StatefulSetNotReady"
	ConditionReasonStatefulSetReady    string = "StatefulSetReady"

	ConditionReasonRestoreNotComplete string = "RestoreNotComplete"
	ConditionReasonRestoreComplete    string = "RestoreComplete"

	ConditionReasonJobComplete  string = "JobComplete"
	ConditionReasonJobSuspended string = "JobSuspended"
	ConditionReasonJobFailed    string = "JobFailed"
	ConditionReasonJobRunning   string = "JobRunning"

	ConditionReasonCreated      string = "Created"
	ConditionReasonProvisioning string = "Provisioning"
	ConditionReasonFailed       string = "Failed"
)
