package conditions

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobConditioner interface {
	AddCondition(condition metav1.Condition)
}

func AddConditionComplete(jb JobConditioner, job *batchv1.Job) {
	switch getJobConditionType(job) {
	case batchv1.JobFailed:
		jb.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonJobFailed,
			Message: "Failed",
		})
	case batchv1.JobComplete:
		jb.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonJobComplete,
			Message: "Success",
		})
	case batchv1.JobSuspended:
		jb.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionFalse,
			Reason:  databasev1alpha1.ConditionReasonJobSuspended,
			Message: "Suspended",
		})
	default:
		jb.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionFalse,
			Reason:  databasev1alpha1.ConditionReasonJobRunning,
			Message: "Running",
		})
	}
}

func getJobConditionType(job *batchv1.Job) batchv1.JobConditionType {
	for _, c := range job.Status.Conditions {
		if c.Status == corev1.ConditionFalse {
			continue
		}
		return c.Type
	}
	return ""
}
