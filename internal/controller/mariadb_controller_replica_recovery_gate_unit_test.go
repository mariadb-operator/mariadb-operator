package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileReplicaRecoveryDisabledKeepsErroredReplicaData(t *testing.T) {
	mariadbBirth := metav1.NewTime(time.Date(2026, 7, 13, 17, 23, 18, 0, time.UTC))
	pvcBirth := metav1.NewTime(time.Date(2026, 7, 13, 17, 23, 20, 0, time.UTC))
	errorTransition := metav1.NewTime(time.Date(2026, 10, 16, 6, 39, 0, 0, time.UTC))

	tests := []struct {
		name         string
		replicaPVC   metav1.Time
		ioErrno      int
		sqlErrno     int
		recoveryOn   bool
		wantRecovery bool
	}{
		{
			name:       "demoted primary failing with duplicate keys is not wiped",
			replicaPVC: pvcBirth,
			sqlErrno:   1062,
		},
		{
			name:       "replica asking for a purged GTID is not wiped",
			replicaPVC: pvcBirth,
			ioErrno:    1236,
		},
		{
			name:         "replica asking for a purged GTID is recovered when recovery is enabled",
			replicaPVC:   pvcBirth,
			ioErrno:      1236,
			recoveryOn:   true,
			wantRecovery: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatalf("error adding MariaDB scheme: %v", err)
			}
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatalf("error adding core scheme: %v", err)
			}

			mariadb := &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "mariadb",
					Namespace:         "test",
					CreationTimestamp: mariadbBirth,
					Annotations: map[string]string{
						storagePVCUIDAnnotationKey(0): "replica-uid",
						storagePVCUIDAnnotationKey(1): "primary-uid",
					},
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 2,
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Primary: mariadbv1alpha1.PrimaryReplication{
								PodIndex: ptr.To(1),
							},
							Replica: mariadbv1alpha1.ReplicaReplication{
								ReplicaRecovery: &mariadbv1alpha1.ReplicaRecovery{
									Enabled: tt.recoveryOn,
								},
							},
						},
					},
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(1),
					Replication: &mariadbv1alpha1.ReplicationStatus{
						Replicas: map[string]mariadbv1alpha1.ReplicaStatus{
							"mariadb-0": {
								LastErrorTransitionTime: errorTransition,
								ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
									LastIOErrno:  ptr.To(tt.ioErrno),
									LastSQLErrno: ptr.To(tt.sqlErrno),
								},
							},
						},
					},
					Conditions: []metav1.Condition{
						{
							Type:   mariadbv1alpha1.ConditionTypeReplicationConfigured,
							Status: metav1.ConditionTrue,
							Reason: mariadbv1alpha1.ConditionReasonReplicationConfigured,
						},
					},
				},
			}
			replicaPVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:              mariadb.PVCKey(builder.StorageVolume, 0).Name,
					Namespace:         mariadb.Namespace,
					UID:               "replica-uid",
					CreationTimestamp: tt.replicaPVC,
				},
			}
			primaryPVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:              mariadb.PVCKey(builder.StorageVolume, 1).Name,
					Namespace:         mariadb.Namespace,
					UID:               "primary-uid",
					CreationTimestamp: pvcBirth,
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
				WithObjects(mariadb, replicaPVC, primaryPVC).
				Build()
			recorder := events.NewFakeRecorder(10)
			reconciler := &MariaDBReconciler{
				Client:   fakeClient,
				Recorder: recorder,
			}

			result, err := reconciler.reconcileReplicaRecovery(context.Background(), mariadb)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var updated mariadbv1alpha1.MariaDB
			if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
				t.Fatalf("error getting MariaDB: %v", err)
			}
			recoveryAttempted := updated.ReplicaRecoveryError() != nil || updated.IsRecoveringReplicas() ||
				len(getActiveReplicaRecoveryReplicas(&updated)) > 0
			if recoveryAttempted != tt.wantRecovery {
				t.Fatalf("unexpected recovery attempt: got %t, want %t (result %+v)", recoveryAttempted, tt.wantRecovery, result)
			}
			if tt.wantRecovery {
				return
			}
			if !result.IsZero() {
				t.Fatalf("expected no requeue for a disabled recovery, got %+v", result)
			}
			select {
			case event := <-recorder.Events:
				if !strings.Contains(event, mariadbv1alpha1.ReasonMariaDBReplicaRecoveryDisabled) {
					t.Fatalf("unexpected event: %s", event)
				}
			default:
				t.Fatal("expected a replica recovery disabled event")
			}
		})
	}
}
