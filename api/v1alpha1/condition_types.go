package v1alpha1

const (
	ConditionTypeReady                 string = "Ready"
	ConditionTypeBackupRestored        string = "BackupRestored"
	ConditionTypeReplicationConfigured string = "ReplicationConfigured"
	ConditionTypePrimarySwitched       string = "PrimarySwitched"
	ConditionTypeComplete              string = "Complete"
)

const (
	ConditionReasonStatefulSetNotReady  string = "StatefulSetNotReady"
	ConditionReasonStatefulSetReady     string = "StatefulSetReady"
	ConditionReasonRestoreBackup        string = "RestoreBackup"
	ConditionReasonConfigureReplication string = "ConfigureReplication"
	ConditionReasonSwitchPrimary        string = "SwitchPrimary"

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

	ConditionReasonCreated string = "Created"
	ConditionReasonHealthy string = "Healthy"
	ConditionReasonFailed  string = "Failed"
)
