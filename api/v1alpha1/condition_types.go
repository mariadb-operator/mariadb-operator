package v1alpha1

const (
	ConditionTypeReady                 string = "Ready"
	ConditionTypeBackupRestored        string = "BackupRestored"
	ConditionTypeReplicationConfigured string = "ReplicationConfigured"
	ConditionTypePrimarySwitched       string = "PrimarySwitched"
	// ConditionTypeGaleraReady indicates that the cluster is healthy.
	ConditionTypeGaleraReady string = "GaleraReady"
	// ConditionTypeGaleraConfigured indicates that the cluster has been successfully configured.
	ConditionTypeGaleraConfigured string = "GaleraConfigured"
	ConditionTypeComplete         string = "Complete"

	ConditionReasonStatefulSetNotReady  string = "StatefulSetNotReady"
	ConditionReasonStatefulSetReady     string = "StatefulSetReady"
	ConditionReasonRestoreBackup        string = "RestoreBackup"
	ConditionReasonConfigureReplication string = "ConfigureReplication"
	ConditionReasonSwitchPrimary        string = "SwitchPrimary"
	ConditionReasonGaleraReady          string = "GaleraReady"
	ConditionReasonGaleraNotReady       string = "GaleraNotReady"
	ConditionReasonGaleraConfigured     string = "GaleraConfigured"

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
