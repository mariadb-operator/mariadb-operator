package controller

import (
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPhysicalBackupJobStatusConditionUsesLatestJob(t *testing.T) {
	baseTime := time.Date(2026, 7, 8, 13, 0, 0, 0, time.UTC)
	backup := &mariadbv1alpha1.PhysicalBackup{
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			Schedule: &mariadbv1alpha1.PhysicalBackupSchedule{
				Cron: "0 2 * * *",
			},
		},
	}

	jobList := &batchv1.JobList{
		Items: []batchv1.Job{
			physicalBackupJob("backup-physical-20260708130000", baseTime, batchv1.JobFailed),
			physicalBackupJob("backup-physical-20260708131500", baseTime.Add(15*time.Minute), batchv1.JobComplete),
		},
	}

	condition := physicalBackupJobStatusCondition(backup, jobList)
	if condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected status %q, got %q", metav1.ConditionTrue, condition.Status)
	}
	if condition.Reason != mariadbv1alpha1.ConditionReasonJobComplete {
		t.Fatalf("expected reason %q, got %q", mariadbv1alpha1.ConditionReasonJobComplete, condition.Reason)
	}
	if condition.Message != "Success" {
		t.Fatalf("expected success message, got %q", condition.Message)
	}
}

func TestPhysicalBackupJobStatusConditionReportsLatestFailure(t *testing.T) {
	baseTime := time.Date(2026, 7, 8, 13, 0, 0, 0, time.UTC)
	backup := &mariadbv1alpha1.PhysicalBackup{}
	jobList := &batchv1.JobList{
		Items: []batchv1.Job{
			physicalBackupJob("backup-physical-20260708130000", baseTime, batchv1.JobComplete),
			physicalBackupJob("backup-physical-20260708131500", baseTime.Add(15*time.Minute), batchv1.JobFailed),
		},
	}

	condition := physicalBackupJobStatusCondition(backup, jobList)
	if condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected status %q, got %q", metav1.ConditionTrue, condition.Status)
	}
	if condition.Reason != mariadbv1alpha1.ConditionReasonJobFailed {
		t.Fatalf("expected reason %q, got %q", mariadbv1alpha1.ConditionReasonJobFailed, condition.Reason)
	}
}

func physicalBackupJob(name string, creationTime time.Time, conditionType batchv1.JobConditionType) batchv1.Job {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(creationTime),
		},
	}
	if conditionType != "" {
		job.Status.Conditions = []batchv1.JobCondition{
			{
				Type:   conditionType,
				Status: corev1.ConditionTrue,
			},
		}
	}
	return job
}
