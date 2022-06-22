package v1alpha1

const (
	ConditionTypeReady                  string = "Ready"
	ConditionReasonStatefulSetNotReady  string = "StatefulSetNotReady"
	ConditionReasonStatefulSetReady     string = "StatefulSetReady"
	ConditionReasonStatefulUnknownState string = "StatefulSetUnknownState"
)

const (
	ConditionTypeComplete       string = "Complete"
	ConditionReasonJobComplete  string = "JobComplete"
	ConditionReasonJobSuspended string = "JobSuspended"
	ConditionReasonJobFailed    string = "JobFailed"
	ConditionReasonJobRunning   string = "JobRunning"
)
