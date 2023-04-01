package v1alpha1

const (
	ConditionTypeReady           string = "Ready"
	ConditionTypeBootstrapped    string = "Bootstrapped"
	ConditionTypePrimarySwitched string = "PrimarySwitched"
	ConditionTypeComplete        string = "Complete"
)

const (
	ConditionReasonStatefulSetNotReady string = "StatefulSetNotReady"
	ConditionReasonStatefulSetReady    string = "StatefulSetReady"
	ConditionReasonSwitchingPrimary    string = "SwitchingPrimary"

	ConditionReasonRestoreNotComplete string = "RestoreNotComplete"
	ConditionReasonRestoreComplete    string = "RestoreComplete"

	ConditionReasonJobComplete  string = "JobComplete"
	ConditionReasonJobSuspended string = "JobSuspended"
	ConditionReasonJobFailed    string = "JobFailed"
	ConditionReasonJobRunning   string = "JobRunning"

	ConditionReasonCronJobScheduled string = "CronJobScheduled"
	ConditionReasonCronJobFailed    string = "CronJobScheduled"
	ConditionReasonCronJobRunning   string = "CronJobRunning"
	ConditionReasonCronJobSuccess   string = "CronJobSucess"

	ConditionReasonConnectionFailed string = "ConnectionFailed"

	ConditionReasonSwitchoverInProgress string = "SwitchoverInProgress"
	ConditionReasonSwitchoverComplete   string = "SwitchoverComplete"

	ConditionReasonCreated string = "Created"
	ConditionReasonHealthy string = "Healthy"
	ConditionReasonFailed  string = "Failed"
)
