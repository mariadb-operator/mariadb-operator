package job

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func IsJobComplete(job *batchv1.Job) bool {
	if job == nil {
		return false
	}
	return IsStatusConditionTrue(job.Status.Conditions, batchv1.JobComplete)
}

func IsStatusConditionTrue(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) bool {
	for _, c := range conditions {
		if c.Type == conditionType && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
