package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
		"unparsable job name": {
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

func TestCleanupReplicaRecoveryArtifactsCleansStorageBeforeDeletingJobs(t *testing.T) {
	const (
		mariadbName = "db-cluster"
		namespace   = "default"
		jobTime     = "20260902070050"
	)
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding batch scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadbName,
			Namespace: namespace,
		},
	}
	backupKey := mariadb.PhysicalBackupReplicaRecoveryKey()
	backup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupKey.Name,
			Namespace: backupKey.Namespace,
			UID:       "recovery-backup-uid",
		},
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			Storage: mariadbv1alpha1.PhysicalBackupStorage{
				AzureBlob: &mariadbv1alpha1.AzureBlob{
					ContainerName:      "backups",
					ServiceURL:         "https://account.blob.core.windows.net/",
					StorageAccountName: "account",
					StorageAccountKey: &mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: "missing-storage-credentials",
						},
						Key: "storage-account-key",
					},
				},
			},
		},
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", backupKey.Name, jobTime),
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: mariadbv1alpha1.GroupVersion.String(),
					Kind:       mariadbv1alpha1.PhysicalBackupKind,
					Name:       backupKey.Name,
					UID:        backup.UID,
					Controller: ptr.To(true),
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, backup, job).
		WithIndex(&batchv1.Job{}, metadata.MetaCtrlFieldPath, func(o client.Object) []string {
			owner := metav1.GetControllerOf(o)
			if owner == nil || owner.Kind != mariadbv1alpha1.PhysicalBackupKind {
				return nil
			}
			return []string{owner.Name}
		}).
		Build()
	recorder := events.NewFakeRecorder(10)
	reconciler := &MariaDBReconciler{
		Client:      fakeClient,
		RefResolver: refresolver.New(fakeClient),
		Recorder:    recorder,
	}

	if err := reconciler.cleanupReplicaRecoveryArtifacts(context.Background(), mariadb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantFile := fmt.Sprintf("physicalbackup-%s.xb", jobTime)
	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, mariadbv1alpha1.ReasonMariaDBReplicaRecoveryStorageLeak) {
			t.Errorf("expected a storage leak event, got %q", event)
		}
		if !strings.Contains(event, wantFile) {
			t.Errorf("expected event to name %q, got %q", wantFile, event)
		}
	default:
		t.Fatal("expected the recovery backup storage to be cleaned up while the Jobs still existed")
	}

	var deleted mariadbv1alpha1.PhysicalBackup
	if err := reconciler.Get(context.Background(), backupKey, &deleted); !apierrors.IsNotFound(err) {
		t.Errorf("expected the recovery PhysicalBackup to be deleted, got %v", err)
	}
}
