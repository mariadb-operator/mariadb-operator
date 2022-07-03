package conditions

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Conditioner interface {
	AddCondition(condition metav1.Condition)
}

func AddConditionReady(c Conditioner, err error) {
	if err == nil {
		c.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonCreated,
			Message: "Created",
		})
	} else {
		c.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  databasev1alpha1.ConditionReasonFailed,
			Message: "Failed",
		})
	}
}

func AddConditionComplete(c Conditioner, job *batchv1.Job) {
	switch getJobConditionType(job) {
	case batchv1.JobFailed:
		c.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonJobFailed,
			Message: "Failed",
		})
	case batchv1.JobComplete:
		c.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonJobComplete,
			Message: "Success",
		})
	case batchv1.JobSuspended:
		c.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionFalse,
			Reason:  databasev1alpha1.ConditionReasonJobSuspended,
			Message: "Suspended",
		})
	default:
		c.AddCondition(metav1.Condition{
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
