package controller

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPreserveReplicaRefusesPrimaryStorageReplacedAfterChecks(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	mariadb.Annotations[storagePVCUIDAnnotationKey(1)] = "primary-uid"
	mariadb.Annotations[replicaRecoveryPreservedAnnotationKey(0)] = preserveTestRecoveryID
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid"), preserveTestPrimaryPVC("fresh-primary-uid")).Build()
	recorder := events.NewFakeRecorder(10)
	r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder, replicaRecoveryGtidsFn: preserveTestHealthyGtids()}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreserveRefusedDelay {
		t.Fatalf("expected the recovery to be refused after the primary storage was replaced, got %+v, %v", result, err)
	}
	expectPrimaryRefusal(t, r, mariadb, recorder, "the storage PVC of the primary db-cluster-1 was replaced")
}

func recheckTestMariaDB(t *testing.T, checked replicaRecoveryCheckedPrimary) *mariadbv1alpha1.MariaDB {
	t.Helper()
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	mariadb.Spec.Replicas = 3
	mariadb.Annotations[storagePVCUIDAnnotationKey(1)] = "primary-uid"
	mariadb.Annotations[replicaRecoveryPreservedAnnotationKey(0)] = preserveTestRecoveryID
	raw, err := json.Marshal(checked)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mariadb.Annotations[replicaRecoveryCheckedPrimaryAnnotationKey(0)] = string(raw)
	return mariadb
}

func recheckTestChecked() replicaRecoveryCheckedPrimary {
	return replicaRecoveryCheckedPrimary{
		Episode:         preserveTestRecoveryID,
		PrimaryIndex:    1,
		PrimaryPVCUID:   "primary-uid",
		ReplicaServerID: preserveTestReplicaServerID,
		ReplicaGtids:    preserveTestReplicaState,
	}
}

func TestRecheckRecoveryPrimary(t *testing.T) {
	tests := []struct {
		name          string
		checked       replicaRecoveryCheckedPrimary
		primaryIndex  int
		primaryState  string
		wantRefused   bool
		wantReason    string
		wantRecorded  int
		wantGtidReads bool
	}{
		{
			name:         "same primary goes on without reading it",
			checked:      recheckTestChecked(),
			primaryIndex: 1,
			wantRecorded: 1,
		},
		{
			name:          "new primary behind the replica is refused",
			checked:       recheckTestChecked(),
			primaryIndex:  2,
			primaryState:  "0-10-400,0-11-3",
			wantRefused:   true,
			wantReason:    "the primary holds domain 0 only up to sequence 400",
			wantRecorded:  1,
			wantGtidReads: true,
		},
		{
			name:          "new primary holding the replica transactions goes on",
			checked:       recheckTestChecked(),
			primaryIndex:  2,
			primaryState:  preserveTestPrimaryState,
			wantRecorded:  2,
			wantGtidReads: true,
		},
		{
			name: "new primary is only read when the replica transactions are unknown",
			checked: replicaRecoveryCheckedPrimary{
				Episode:       preserveTestRecoveryID,
				PrimaryIndex:  1,
				PrimaryPVCUID: "primary-uid",
			},
			primaryIndex:  2,
			primaryState:  "0-10-1",
			wantRecorded:  2,
			wantGtidReads: true,
		},
		{
			name: "checks of another episode only recheck the primary storage",
			checked: replicaRecoveryCheckedPrimary{
				Episode:      "replica-uid-1",
				PrimaryIndex: 1,
			},
			primaryIndex: 2,
			wantRecorded: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mariadb := recheckTestMariaDB(t, tt.checked)
			mariadb.Status.CurrentPrimaryPodIndex = ptr.To(tt.primaryIndex)
			objects := []client.Object{mariadb, preserveTestPVC(mariadb, "replica-uid"), preserveTestPrimaryPVC("primary-uid")}
			fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).WithObjects(objects...).Build()
			recorder := events.NewFakeRecorder(10)
			gtidReads := 0
			r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder,
				replicaRecoveryGtidsFn: func(context.Context, *mariadbv1alpha1.MariaDB, int) (*replicaRecoveryGtids, error) {
					gtidReads++
					return &replicaRecoveryGtids{PrimaryBinlogState: tt.primaryState}, nil
				}}

			result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantRefused != (result.RequeueAfter == replicaRecoveryPreserveRefusedDelay) {
				t.Fatalf("unexpected result: %+v", result)
			}
			if tt.wantRefused {
				expectPrimaryRefusal(t, r, mariadb, recorder, tt.wantReason)
			} else if !result.IsZero() {
				t.Fatalf("expected the recovery to go on, got %+v", result)
			}
			if tt.wantGtidReads != (gtidReads > 0) {
				t.Fatalf("unexpected primary GTID reads: %d", gtidReads)
			}

			var stored mariadbv1alpha1.MariaDB
			if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &stored); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var recorded replicaRecoveryCheckedPrimary
			if err := json.Unmarshal([]byte(stored.Annotations[replicaRecoveryCheckedPrimaryAnnotationKey(0)]), &recorded); err != nil {
				t.Fatalf("unexpected error decoding checked primary: %v", err)
			}
			if recorded.PrimaryIndex != tt.wantRecorded {
				t.Fatalf("expected the checked primary to be %d, got %+v", tt.wantRecorded, recorded)
			}
			if tt.checked.Episode == preserveTestRecoveryID && recorded.ReplicaGtids != tt.checked.ReplicaGtids {
				t.Fatalf("expected the replica transactions to be kept, got %+v", recorded)
			}
		})
	}
}

func TestPreserveReplicaRecordsCheckedPrimary(t *testing.T) {
	mariadb := preserveTestMariaDB(nil)
	mariadb.Annotations[storagePVCUIDAnnotationKey(1)] = "primary-uid"
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid"), preserveTestPrimaryPVC("primary-uid")).Build()
	r := &MariaDBReconciler{Client: fakeClient, replicaRecoveryGtidsFn: preserveTestHealthyGtids()}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || !result.IsZero() {
		t.Fatalf("expected the recovery to go on, got %+v, %v", result, err)
	}
	var stored mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &stored); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := checkedRecoveryPrimary(&stored, 0, preserveTestRecoveryID)
	want := &replicaRecoveryCheckedPrimary{
		Episode:         preserveTestRecoveryID,
		PrimaryIndex:    1,
		PrimaryPVCUID:   "primary-uid",
		ReplicaServerID: preserveTestReplicaServerID,
		ReplicaGtids:    preserveTestReplicaState + ",",
	}
	if got == nil || *got != *want {
		t.Fatalf("unexpected checked primary: got %+v, want %+v", got, want)
	}
}
