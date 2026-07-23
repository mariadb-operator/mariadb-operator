package init

import (
	"context"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newReplicationTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
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
	return scheme
}

func newReplicationTestMariadb(replicaToRecover string, annotations map[string]string) *mariadbv1alpha1.MariaDB {
	mdb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mariadb",
			Namespace:   "test",
			Annotations: annotations,
		},
	}
	if replicaToRecover != "" {
		mdb.Status.Replication = &mariadbv1alpha1.ReplicationStatus{
			ReplicaToRecover: &replicaToRecover,
		}
	}
	return mdb
}

func newRestoreJob(mdb *mariadbv1alpha1.MariaDB, podIndex int, complete bool, pvcUID string,
	creationTimestamp time.Time) *batchv1.Job {
	key := mdb.PhysicalBackupInitJobKey(podIndex)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              key.Name,
			Namespace:         key.Namespace,
			CreationTimestamp: metav1.NewTime(creationTimestamp),
		},
	}
	if pvcUID != "" {
		job.Annotations = map[string]string{
			metadata.InitJobStoragePVCUIDAnnotation: pvcUID,
		}
	}
	if complete {
		job.Status.Conditions = []batchv1.JobCondition{
			{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
			},
		}
	}
	return job
}

func newStoragePVC(mdb *mariadbv1alpha1.MariaDB, podIndex int, uid string,
	creationTimestamp time.Time) *corev1.PersistentVolumeClaim {
	key := mdb.PVCKey(builder.StorageVolume, podIndex)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              key.Name,
			Namespace:         key.Namespace,
			UID:               types.UID(uid),
			CreationTimestamp: metav1.NewTime(creationTimestamp),
		},
	}
}

func TestIsReplicaRestoreComplete(t *testing.T) {
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		annotations map[string]string
		objects     func(mdb *mariadbv1alpha1.MariaDB) []ctrlclient.Object
		want        bool
		wantErr     bool
	}{
		{
			name: "no storage PVC",
			objects: func(*mariadbv1alpha1.MariaDB) []ctrlclient.Object {
				return nil
			},
			wantErr: true,
		},
		{
			name: "no restore Job",
			objects: func(mdb *mariadbv1alpha1.MariaDB) []ctrlclient.Object {
				return []ctrlclient.Object{
					newStoragePVC(mdb, 0, "pvc-uid", baseTime),
				}
			},
			want: false,
		},
		{
			name: "restore Job not complete",
			objects: func(mdb *mariadbv1alpha1.MariaDB) []ctrlclient.Object {
				return []ctrlclient.Object{
					newRestoreJob(mdb, 0, false, "pvc-uid", baseTime.Add(time.Hour)),
					newStoragePVC(mdb, 0, "pvc-uid", baseTime),
				}
			},
			want: false,
		},
		{
			name: "restore Job complete for current PVC",
			objects: func(mdb *mariadbv1alpha1.MariaDB) []ctrlclient.Object {
				return []ctrlclient.Object{
					newRestoreJob(mdb, 0, true, "pvc-uid", baseTime.Add(time.Hour)),
					newStoragePVC(mdb, 0, "pvc-uid", baseTime),
				}
			},
			want: true,
		},
		{
			name: "restore Job complete for replaced PVC",
			objects: func(mdb *mariadbv1alpha1.MariaDB) []ctrlclient.Object {
				return []ctrlclient.Object{
					newRestoreJob(mdb, 0, true, "old-pvc-uid", baseTime.Add(time.Hour)),
					newStoragePVC(mdb, 0, "new-pvc-uid", baseTime),
				}
			},
			want: false,
		},
		{
			name: "legacy restore Job without annotation newer than PVC",
			objects: func(mdb *mariadbv1alpha1.MariaDB) []ctrlclient.Object {
				return []ctrlclient.Object{
					newRestoreJob(mdb, 0, true, "", baseTime.Add(time.Hour)),
					newStoragePVC(mdb, 0, "pvc-uid", baseTime),
				}
			},
			want: true,
		},
		{
			name: "legacy restore Job without annotation older than PVC",
			objects: func(mdb *mariadbv1alpha1.MariaDB) []ctrlclient.Object {
				return []ctrlclient.Object{
					newRestoreJob(mdb, 0, true, "", baseTime),
					newStoragePVC(mdb, 0, "pvc-uid", baseTime.Add(time.Hour)),
				}
			},
			want: false,
		},
		{
			name: "completed PVC annotation matches current PVC",
			annotations: map[string]string{
				metadata.ReplicaRecoveryCompletedPVCUIDAnnotationKey(0): "pvc-uid",
			},
			objects: func(mdb *mariadbv1alpha1.MariaDB) []ctrlclient.Object {
				return []ctrlclient.Object{
					newStoragePVC(mdb, 0, "pvc-uid", baseTime),
				}
			},
			want: true,
		},
		{
			name: "completed PVC annotation for replaced PVC",
			annotations: map[string]string{
				metadata.ReplicaRecoveryCompletedPVCUIDAnnotationKey(0): "old-pvc-uid",
			},
			objects: func(mdb *mariadbv1alpha1.MariaDB) []ctrlclient.Object {
				return []ctrlclient.Object{
					newStoragePVC(mdb, 0, "new-pvc-uid", baseTime),
				}
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testMdb := newReplicationTestMariadb("mariadb-0", tt.annotations)
			fakeClient := fake.NewClientBuilder().
				WithScheme(newReplicationTestScheme(t)).
				WithObjects(tt.objects(testMdb)...).
				Build()

			got, err := isReplicaRestoreComplete(context.Background(), testMdb, 0, fakeClient)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestWaitForReplicaRecoveryUnblocksOnCompletedRestoreJob(t *testing.T) {
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mdb := newReplicationTestMariadb("mariadb-0", nil)
	fakeClient := fake.NewClientBuilder().
		WithScheme(newReplicationTestScheme(t)).
		WithObjects(
			mdb,
			newRestoreJob(mdb, 0, true, "pvc-uid", baseTime.Add(time.Hour)),
			newStoragePVC(mdb, 0, "pvc-uid", baseTime),
		).
		Build()
	env := &environment.PodEnvironment{
		PodName:      "mariadb-0",
		PodNamespace: "test",
		MariadbName:  "mariadb",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := waitForReplicaRecovery(ctx, env, mdb, 0, fakeClient); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForReplicaRecoveryUnblocksOnCompletedPVCAnnotation(t *testing.T) {
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mdb := newReplicationTestMariadb("mariadb-0", map[string]string{
		metadata.ReplicaRecoveryCompletedPVCUIDAnnotationKey(0): "pvc-uid",
	})
	fakeClient := fake.NewClientBuilder().
		WithScheme(newReplicationTestScheme(t)).
		WithObjects(
			mdb,
			newStoragePVC(mdb, 0, "pvc-uid", baseTime),
		).
		Build()
	env := &environment.PodEnvironment{
		PodName:      "mariadb-0",
		PodNamespace: "test",
		MariadbName:  "mariadb",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := waitForReplicaRecovery(ctx, env, mdb, 0, fakeClient); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForReplicaRecoverySkipsWhenNotBeingRecovered(t *testing.T) {
	mdb := newReplicationTestMariadb("", nil)
	fakeClient := fake.NewClientBuilder().
		WithScheme(newReplicationTestScheme(t)).
		Build()
	env := &environment.PodEnvironment{
		PodName:      "mariadb-0",
		PodNamespace: "test",
		MariadbName:  "mariadb",
	}

	if err := waitForReplicaRecovery(context.Background(), env, mdb, 0, fakeClient); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
