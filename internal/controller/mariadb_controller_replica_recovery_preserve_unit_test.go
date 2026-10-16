package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHoldsUnreplicatedOwnTransactions(t *testing.T) {
	tests := []struct {
		name         string
		serverID     uint32
		nodeBinlog   string
		primaryState string
		primarySlave string
		want         bool
		wantErr      bool
	}{
		{
			name:         "demoted primary whose writes all reached the new primary",
			serverID:     11,
			nodeBinlog:   "0-11-22825293",
			primaryState: "0-11-22825293,0-10-22868939",
			want:         false,
		},
		{
			name:         "demoted primary holding writes the new primary never received",
			serverID:     11,
			nodeBinlog:   "0-11-22825301",
			primaryState: "0-11-22825293,0-10-22868939",
			want:         true,
		},
		{
			name:         "diverged node wrote a sequence the primary filled with another server",
			serverID:     11,
			nodeBinlog:   "0-11-500",
			primaryState: "0-10-900",
			want:         true,
		},
		{
			name:         "replica binary log only holds replicated transactions",
			serverID:     11,
			nodeBinlog:   "0-10-900",
			primaryState: "0-10-950",
			want:         false,
		},
		{
			name:         "primary records the node writes through gtid_slave_pos only",
			serverID:     11,
			nodeBinlog:   "0-11-700",
			primaryState: "0-10-900",
			primarySlave: "0-11-700",
			want:         false,
		},
		{
			name:     "empty binary log holds nothing",
			serverID: 11,
			want:     false,
		},
		{
			name:       "malformed GTID is an error",
			serverID:   11,
			nodeBinlog: "0-11",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := holdsUnreplicatedOwnTransactions(tt.serverID, tt.nodeBinlog, tt.primaryState, tt.primarySlave)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected result: got %t, want %t", got, tt.want)
			}
		})
	}
}

const (
	preserveTestClass      = "longhorn-b2-bak"
	preserveTestRecoveryID = "6f1d0a52-3c1e-4e57-9d55-0e7a3f7f5c11"
)

func preserveTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		mariadbv1alpha1.AddToScheme,
		corev1.AddToScheme,
		batchv1.AddToScheme,
		volumesnapshotv1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatalf("error adding scheme: %v", err)
		}
	}
	return scheme
}

func preserveTestMariaDB(className *string) *mariadbv1alpha1.MariaDB {
	return &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-cluster",
			Namespace: "darkflare",
			Annotations: map[string]string{
				storagePVCUIDAnnotationKey(0): "replica-uid",
			},
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Replica: mariadbv1alpha1.ReplicaReplication{
						ReplicaRecovery: &mariadbv1alpha1.ReplicaRecovery{
							Enabled:                         true,
							PreserveVolumeSnapshotClassName: className,
						},
					},
				},
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(1),
		},
	}
}

func preserveTestPVC(mariadb *mariadbv1alpha1.MariaDB, uid types.UID) *corev1.PersistentVolumeClaim {
	key := mariadb.PVCKey(builder.StorageVolume, 0)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			UID:       uid,
		},
	}
}

func preserveTestSnapshot(mariadb *mariadbv1alpha1.MariaDB, name, recoveryID string, created time.Time,
	status *volumesnapshotv1.VolumeSnapshotStatus) *volumesnapshotv1.VolumeSnapshot {
	snapshot := buildReplicaRecoveryPreserveSnapshot(mariadb, preserveTestPVC(mariadb, "replica-uid"), 0,
		recoveryID, preserveTestClass, created)
	snapshot.Name = name
	snapshot.CreationTimestamp = metav1.NewTime(created)
	snapshot.Status = status
	return snapshot
}

func preserveTestRecoveryBackup() *mariadbv1alpha1.PhysicalBackup {
	return &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{Name: "db-cluster-pb-recovery", Namespace: "darkflare", UID: preserveTestRecoveryID},
	}
}

var preserveTestBase = time.Date(2026, 10, 16, 6, 0, 0, 0, time.UTC)

func TestPreserveReplicaSnapshotsStorageBeforeWipe(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	recorder := events.NewFakeRecorder(10)
	r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", preserveTestRecoveryBackup(), logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != replicaRecoveryPreservePollDelay {
		t.Fatalf("expected the wipe to wait for the snapshot, got %+v", result)
	}
	snapshots, err := r.listReplicaRecoveryPreserveSnapshots(context.Background(), mariadb, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one preserve snapshot, got %d", len(snapshots))
	}
	snapshot := snapshots[0]
	if ptr.Deref(snapshot.Spec.Source.PersistentVolumeClaimName, "") != "storage-db-cluster-0" ||
		ptr.Deref(snapshot.Spec.VolumeSnapshotClassName, "") != preserveTestClass ||
		snapshot.Labels[replicaRecoveryPreserveIDLabel] != preserveTestRecoveryID {
		t.Fatalf("unexpected preserve snapshot: %+v", snapshot)
	}
	if event := <-recorder.Events; !strings.Contains(event, mariadbv1alpha1.ReasonMariaDBReplicaRecoveryPreserved) {
		t.Fatalf("unexpected event: %s", event)
	}
}

func TestPreserveReplicaWaitsForSnapshot(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid"),
			preserveTestSnapshot(mariadb, "db-cluster-preserve-0-a", preserveTestRecoveryID, preserveTestBase, nil)).Build()
	r := &MariaDBReconciler{Client: fakeClient}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", preserveTestRecoveryBackup(), logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreservePollDelay {
		t.Fatalf("expected to keep waiting, got %+v, %v", result, err)
	}
}

func TestPreserveReplicaFailedSnapshotBlocksWipe(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	failed := &volumesnapshotv1.VolumeSnapshotStatus{
		Error: &volumesnapshotv1.VolumeSnapshotError{Message: ptr.To("backup target unreachable")},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid"),
			preserveTestSnapshot(mariadb, "db-cluster-preserve-0-a", preserveTestRecoveryID, preserveTestBase, failed)).Build()
	recorder := events.NewFakeRecorder(10)
	r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", preserveTestRecoveryBackup(), logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreserveRetryDelay {
		t.Fatalf("expected the wipe to be blocked, got %+v, %v", result, err)
	}
	if event := <-recorder.Events; !strings.Contains(event, mariadbv1alpha1.ReasonMariaDBReplicaRecoveryRefused) {
		t.Fatalf("unexpected event: %s", event)
	}
}

func TestPreserveReplicaReadySnapshotPrunesOldest(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	objects := []client.Object{mariadb, preserveTestPVC(mariadb, "replica-uid")}
	for i, name := range []string{"db-cluster-preserve-0-1", "db-cluster-preserve-0-2", "db-cluster-preserve-0-3"} {
		created := preserveTestBase.Add(time.Duration(i) * time.Hour)
		objects = append(objects, preserveTestSnapshot(mariadb, name, "older-recovery", created, preserveTestReady()))
	}
	objects = append(objects,
		preserveTestSnapshot(mariadb, "db-cluster-preserve-0-4", preserveTestRecoveryID, preserveTestBase.Add(5*time.Hour), preserveTestReady()))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).WithObjects(objects...).Build()
	r := &MariaDBReconciler{Client: fakeClient}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", preserveTestRecoveryBackup(), logr.Discard())
	if err != nil || !result.IsZero() {
		t.Fatalf("expected the recovery to go on, got %+v, %v", result, err)
	}
	snapshots, err := r.listReplicaRecoveryPreserveSnapshots(context.Background(), mariadb, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := map[string]bool{}
	for _, s := range snapshots {
		names[s.Name] = true
	}
	if len(snapshots) != replicaRecoveryPreserveKeep || names["db-cluster-preserve-0-1"] || !names["db-cluster-preserve-0-4"] {
		t.Fatalf("unexpected snapshots after pruning: %v", names)
	}
}

func TestPreserveReplicaSkipsRecreatedPVC(t *testing.T) {
	mariadb := preserveTestMariaDB(nil)
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "recreated-uid")).Build()
	r := &MariaDBReconciler{Client: fakeClient}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", preserveTestRecoveryBackup(), logr.Discard())
	if err != nil || !result.IsZero() {
		t.Fatalf("expected the recovery to go on, got %+v, %v", result, err)
	}
}

func TestPreserveReplicaSkipsStartedRestore(t *testing.T) {
	mariadb := preserveTestMariaDB(nil)
	initJobKey := mariadb.PhysicalBackupInitJobKey(0)
	initJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: initJobKey.Name, Namespace: initJobKey.Namespace}}
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid"), initJob).Build()
	r := &MariaDBReconciler{Client: fakeClient}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", preserveTestRecoveryBackup(), logr.Discard())
	if err != nil || !result.IsZero() {
		t.Fatalf("expected the recovery to go on, got %+v, %v", result, err)
	}
}

func TestPreserveReplicaRefusesUnverifiableReplica(t *testing.T) {
	mariadb := preserveTestMariaDB(nil)
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	recorder := events.NewFakeRecorder(10)
	r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder, RefResolver: refresolver.New(fakeClient)}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", preserveTestRecoveryBackup(), logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreserveRefusedDelay {
		t.Fatalf("expected the recovery to be refused, got %+v, %v", result, err)
	}
	if event := <-recorder.Events; !strings.Contains(event, mariadbv1alpha1.ReasonMariaDBReplicaRecoveryRefused) {
		t.Fatalf("unexpected event: %s", event)
	}
}

func preserveTestReady() *volumesnapshotv1.VolumeSnapshotStatus {
	return &volumesnapshotv1.VolumeSnapshotStatus{ReadyToUse: ptr.To(true)}
}
