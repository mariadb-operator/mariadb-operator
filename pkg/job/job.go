package job

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func ListJobsForMariaDB(ctx context.Context, client ctrlclient.Client, mariadb *mariadbv1alpha1.MariaDB) (*batchv1.JobList, error) {
	var jobList batchv1.JobList
	if err := client.List(
		ctx,
		&jobList,
		ctrlclient.InNamespace(mariadb.Namespace),
		ctrlclient.MatchingLabels(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mariadb).
				Build(),
		),
	); err != nil {
		return nil, fmt.Errorf("error listing jobs with MariaDB (%s/%s): %v", mariadb.GetNamespace(), mariadb.GetName(), err)
	}

	return &jobList, nil
}

func ListJobs(ctx context.Context, client ctrlclient.Client,
	backup *mariadbv1alpha1.PhysicalBackup) (*batchv1.JobList, error) {
	var jobList batchv1.JobList
	if err := client.List(
		ctx,
		&jobList,
		ctrlclient.InNamespace(backup.Namespace),
		ctrlclient.MatchingFields{metadata.MetaCtrlFieldPath: backup.Name},
	); err != nil {
		return nil, err
	}
	return &jobList, nil
}

func IsJobFailed(job *batchv1.Job) bool {
	if job == nil {
		return false
	}
	return IsStatusConditionTrue(job.Status.Conditions, batchv1.JobFailed)
}

func IsJobSuspended(job *batchv1.Job) bool {
	if job == nil {
		return false
	}
	return IsStatusConditionTrue(job.Status.Conditions, batchv1.JobSuspended)
}

func IsJobComplete(job *batchv1.Job) bool {
	if job == nil {
		return false
	}
	return IsStatusConditionTrue(job.Status.Conditions, batchv1.JobComplete)
}

func IsJobRunning(job *batchv1.Job) bool {
	if job == nil {
		return false
	}
	return !IsJobFailed(job) && !IsJobSuspended(job) && !IsJobComplete(job)
}

func HasRunningJobs(jobList *batchv1.JobList) bool {
	for _, job := range jobList.Items {
		if job.Status.Active > 0 {
			return true
		}
	}

	return false
}

func IsStatusConditionTrue(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) bool {
	for _, c := range conditions {
		if c.Type == conditionType && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
