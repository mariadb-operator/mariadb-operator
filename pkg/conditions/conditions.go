package conditions

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Conditioner interface {
	SetCondition(condition metav1.Condition)
}

type ConditionPatcher func(Conditioner)

func NewConditionCreatedPatcher(err error) ConditionPatcher {
	return func(c Conditioner) {
		if err == nil {
			SetConditionCreated(c)
		} else {
			SetConditionFailed(c)
		}
	}

}

func SetConditionCreatedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    databasev1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  databasev1alpha1.ConditionReasonCreated,
		Message: message,
	})
}

func SetConditionCreated(c Conditioner) {
	SetConditionCreatedWithMessage(c, "Created")
}

func SetConditionProvisioningWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    databasev1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  databasev1alpha1.ConditionReasonProvisioning,
		Message: message,
	})
}

func SetConditionProvisioning(c Conditioner) {
	SetConditionProvisioningWithMessage(c, "Provisioning")
}

func SetConditionFailedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    databasev1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  databasev1alpha1.ConditionReasonFailed,
		Message: message,
	})
}

func SetConditionFailed(c Conditioner) {
	SetConditionFailedWithMessage(c, "Failed")
}

func SetConditionCompleteWithJob(c Conditioner, job *batchv1.Job) {
	switch getJobConditionType(job) {
	case batchv1.JobFailed:
		c.SetCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonJobFailed,
			Message: "Failed",
		})
	case batchv1.JobComplete:
		c.SetCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonJobComplete,
			Message: "Success",
		})
	case batchv1.JobSuspended:
		c.SetCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionFalse,
			Reason:  databasev1alpha1.ConditionReasonJobSuspended,
			Message: "Suspended",
		})
	default:
		c.SetCondition(metav1.Condition{
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
