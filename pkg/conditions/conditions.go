package conditions

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Conditioner interface {
	SetCondition(condition metav1.Condition)
}

func SetReadyHealthty(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonHealthy,
		Message: "Healthy",
	})
}

func SetReadyUnhealthtyWithError(c Conditioner, err error) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonHealthy,
		Message: err.Error(),
	})
}

func SetReadyCreatedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonCreated,
		Message: message,
	})
}

func SetReadyCreated(c Conditioner) {
	SetReadyCreatedWithMessage(c, "Created")
}

func SetReadyFailedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonFailed,
		Message: message,
	})
}

func SetReadyFailed(c Conditioner) {
	SetReadyFailedWithMessage(c, "Failed")
}

func SetReadySwitchingPrimary(c Conditioner, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Replication == nil {
		return errors.New("replication required to set switching primary Ready condition")
	}
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonSwitchingPrimary,
		Message: switchingPrimaryMessage(mariadb),
	})
	return nil
}

func SetPrimarySwitchedInProgress(c Conditioner, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Replication == nil {
		return errors.New("replication required to set in progress PrimarySwitched condition")
	}
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypePrimarySwitched,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonSwitchoverInProgress,
		Message: switchingPrimaryMessage(mariadb),
	})
	return nil
}

func SetPrimarySwitchedComplete(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypePrimarySwitched,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonSwitchoverComplete,
		Message: "Switchover complete",
	})
}

func SetCompleteWithCronJob(c Conditioner, cronJob *batchv1.CronJob) {
	setScheduled := func() {
		c.SetCondition(metav1.Condition{
			Type:    mariadbv1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionFalse,
			Reason:  mariadbv1alpha1.ConditionReasonCronJobScheduled,
			Message: "Scheduled",
		})
	}

	if cronJob.Status.LastScheduleTime == nil || cronJob.Status.LastSuccessfulTime == nil {
		setScheduled()
		return
	}
	if cronJob.Status.LastSuccessfulTime.Before(cronJob.Status.LastScheduleTime) {
		if len(cronJob.Status.Active) > 0 {
			c.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonCronJobRunning,
				Message: "Running",
			})
		} else {
			c.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonCronJobFailed,
				Message: "Failed",
			})
		}
		return
	}

	if cronJob.Status.LastScheduleTime.Equal(cronJob.Status.LastSuccessfulTime) ||
		cronJob.Status.LastScheduleTime.Before(cronJob.Status.LastSuccessfulTime) {
		c.SetCondition(metav1.Condition{
			Type:    mariadbv1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionTrue,
			Reason:  mariadbv1alpha1.ConditionReasonCronJobSuccess,
			Message: "Success",
		})
		return
	}

	setScheduled()
}

func SetCompleteWithJob(c Conditioner, job *batchv1.Job) {
	switch getJobConditionType(job) {
	case batchv1.JobFailed:
		c.SetCondition(metav1.Condition{
			Type:    mariadbv1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionTrue,
			Reason:  mariadbv1alpha1.ConditionReasonJobFailed,
			Message: "Failed",
		})
	case batchv1.JobComplete:
		c.SetCondition(metav1.Condition{
			Type:    mariadbv1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionTrue,
			Reason:  mariadbv1alpha1.ConditionReasonJobComplete,
			Message: "Success",
		})
	case batchv1.JobSuspended:
		c.SetCondition(metav1.Condition{
			Type:    mariadbv1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionFalse,
			Reason:  mariadbv1alpha1.ConditionReasonJobSuspended,
			Message: "Suspended",
		})
	default:
		c.SetCondition(metav1.Condition{
			Type:    mariadbv1alpha1.ConditionTypeComplete,
			Status:  metav1.ConditionFalse,
			Reason:  mariadbv1alpha1.ConditionReasonJobRunning,
			Message: "Running",
		})
	}
}

func SetCompleteFailedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeComplete,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonFailed,
		Message: message,
	})
}

func SetCompleteFailed(c Conditioner) {
	SetCompleteFailedWithMessage(c, "Failed")
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

func switchingPrimaryMessage(mariadb *mariadbv1alpha1.MariaDB) string {
	return fmt.Sprintf(
		"Switching primary to '%s'",
		statefulset.PodName(mariadb.ObjectMeta, mariadb.Spec.Replication.Primary.PodIndex),
	)
}
