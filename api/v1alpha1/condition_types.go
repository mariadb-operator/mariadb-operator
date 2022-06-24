package v1alpha1

const (
	ConditionTypeReady                  string = "Ready"
	ConditionTypeBootstrapped           string = "Bootstrapped"
	ConditionReasonStatefulSetNotReady  string = "StatefulSetNotReady"
	ConditionReasonStatefulSetReady     string = "StatefulSetReady"
	ConditionReasonStatefulUnknownState string = "StatefulSetUnknownState"
	ConditionReasonRestoreNotComplete   string = "RestoreNotComplete"
	ConditionReasonRestoreComplete      string = "RestoreComplete"
)

const (
	ConditionTypeComplete       string = "Complete"
	ConditionReasonJobComplete  string = "JobComplete"
	ConditionReasonJobSuspended string = "JobSuspended"
	ConditionReasonJobFailed    string = "JobFailed"
	ConditionReasonJobRunning   string = "JobRunning"
)
