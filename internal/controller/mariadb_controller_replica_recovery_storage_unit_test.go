package controller

import (
	"slices"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReplicaRecoveryBackupFileNames(t *testing.T) {
	newJobList := func(names ...string) *batchv1.JobList {
		jobList := &batchv1.JobList{}
		for _, name := range names {
			jobList.Items = append(jobList.Items, batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
			})
		}
		return jobList
	}
	newBackup := func(compression mariadbv1alpha1.CompressAlgorithm) *mariadbv1alpha1.PhysicalBackup {
		return &mariadbv1alpha1.PhysicalBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db-cluster-pb-recovery",
				Namespace: "default",
			},
			Spec: mariadbv1alpha1.PhysicalBackupSpec{
				Compression: compression,
			},
		}
	}

	testCases := map[string]struct {
		backup  *mariadbv1alpha1.PhysicalBackup
		jobList *batchv1.JobList
		want    []string
		wantErr bool
	}{
		"single recovery backup": {
			backup:  newBackup(mariadbv1alpha1.CompressNone),
			jobList: newJobList("db-cluster-pb-recovery-20260902070050"),
			want:    []string{"physicalbackup-20260902070050.xb"},
		},
		"retried recovery leaves several backups": {
			backup: newBackup(mariadbv1alpha1.CompressNone),
			jobList: newJobList(
				"db-cluster-pb-recovery-20260902070050",
				"db-cluster-pb-recovery-20260902081500",
			),
			want: []string{
				"physicalbackup-20260902070050.xb",
				"physicalbackup-20260902081500.xb",
			},
		},
		"compressed recovery backup": {
			backup:  newBackup(mariadbv1alpha1.CompressGzip),
			jobList: newJobList("db-cluster-pb-recovery-20260902070050"),
			want:    []string{"physicalbackup-20260902070050.xb.gz"},
		},
		"no jobs": {
			backup:  newBackup(mariadbv1alpha1.CompressNone),
			jobList: newJobList(),
			want:    []string{},
		},
		"nil job list": {
			backup:  newBackup(mariadbv1alpha1.CompressNone),
			jobList: nil,
			want:    nil,
		},
		"unparseable job name": {
			backup:  newBackup(mariadbv1alpha1.CompressNone),
			jobList: newJobList("db-cluster-pb-recovery-nope"),
			wantErr: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := replicaRecoveryBackupFileNames(tc.backup, tc.jobList)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
