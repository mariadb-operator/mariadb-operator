package controller

import (
	"context"
	"errors"
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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
	preserveTestClass         = "longhorn-b2-bak"
	preserveTestRecoveryStart = 1792126800
	preserveTestRecoveryID    = "replica-uid-1792126800"
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
			Conditions: []metav1.Condition{
				{
					Type:               mariadbv1alpha1.ConditionTypeReplicaRecovered,
					Status:             metav1.ConditionFalse,
					Reason:             mariadbv1alpha1.ConditionReasonReplicaRecovered,
					LastTransitionTime: metav1.NewTime(time.Unix(preserveTestRecoveryStart, 0)),
				},
			},
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

var preserveTestBase = time.Date(2026, 10, 16, 6, 0, 0, 0, time.UTC)

func TestPreserveReplicaSnapshotsStorageBeforeWipe(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	recorder := events.NewFakeRecorder(10)
	r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder, replicaRecoveryGtidsFn: preserveTestHealthyGtids()}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
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
	r := &MariaDBReconciler{Client: fakeClient, replicaRecoveryGtidsFn: preserveTestHealthyGtids()}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
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
	r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder, replicaRecoveryGtidsFn: preserveTestHealthyGtids()}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
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
	r := &MariaDBReconciler{Client: fakeClient, replicaRecoveryGtidsFn: preserveTestHealthyGtids()}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
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

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
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

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || !result.IsZero() {
		t.Fatalf("expected the recovery to go on, got %+v, %v", result, err)
	}
}

func TestPreserveReplicaRefusesUnverifiableReplica(t *testing.T) {
	mariadb := preserveTestMariaDB(nil)
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	recorder := events.NewFakeRecorder(10)
	r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder,
		replicaRecoveryGtidsFn: preserveTestGtids(preserveTestPrimaryState, "", errors.New("connection refused"))}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreserveRefusedDelay {
		t.Fatalf("expected the recovery to be refused, got %+v, %v", result, err)
	}
	if event := <-recorder.Events; !strings.Contains(event, mariadbv1alpha1.ReasonMariaDBReplicaRecoveryRefused) ||
		!strings.Contains(event, "preserveVolumeSnapshotClassName") {
		t.Fatalf("unexpected event: %s", event)
	}
}

func TestPreserveReplicaRefusesWithoutPrimaryConnection(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	recorder := events.NewFakeRecorder(10)
	r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder, RefResolver: refresolver.New(fakeClient)}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreserveRefusedDelay {
		t.Fatalf("expected the recovery to be refused, got %+v, %v", result, err)
	}
	expectPrimaryRefusal(t, r, mariadb, recorder, "the primary could not be checked")
}

func TestPreserveReplicaRefusesReplacedPrimaryStorage(t *testing.T) {
	tests := []struct {
		name       string
		primaryPVC *corev1.PersistentVolumeClaim
	}{
		{
			name:       "recreated primary PVC",
			primaryPVC: preserveTestPrimaryPVC("fresh-primary-uid"),
		},
		{
			name: "missing primary PVC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
			mariadb.Annotations[storagePVCUIDAnnotationKey(1)] = "primary-uid"
			objects := []client.Object{mariadb, preserveTestPVC(mariadb, "replica-uid")}
			if tt.primaryPVC != nil {
				objects = append(objects, tt.primaryPVC)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).WithObjects(objects...).Build()
			recorder := events.NewFakeRecorder(10)
			r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder,
				replicaRecoveryGtidsFn: func(context.Context, *mariadbv1alpha1.MariaDB, int) (*replicaRecoveryGtids, error) {
					t.Fatal("expected the replaced primary storage to be refused before reading GTIDs")
					return nil, nil
				}}

			result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
			if err != nil || result.RequeueAfter != replicaRecoveryPreserveRefusedDelay {
				t.Fatalf("expected the recovery to be refused, got %+v, %v", result, err)
			}
			expectPrimaryRefusal(t, r, mariadb, recorder, "the storage PVC of the primary db-cluster-1 was replaced")
		})
	}
}

func TestPreserveReplicaRefusesPrimaryBehindReplica(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	recorder := events.NewFakeRecorder(10)
	r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder,
		replicaRecoveryGtidsFn: preserveTestGtids("0-11-3", preserveTestReplicaState, nil)}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreserveRefusedDelay {
		t.Fatalf("expected the recovery to be refused, got %+v, %v", result, err)
	}
	expectPrimaryRefusal(t, r, mariadb, recorder, "the primary holds domain 0 only up to sequence 3, but the replica holds 0-11-850")
}

func TestPreserveReplicaSnapshotsUnreadableReplica(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	r := &MariaDBReconciler{Client: fakeClient,
		replicaRecoveryGtidsFn: preserveTestGtids(preserveTestPrimaryState, "", errors.New("connection refused"))}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreservePollDelay {
		t.Fatalf("expected the replica to be snapshotted, got %+v, %v", result, err)
	}
	expectPreserveSnapshots(t, r, mariadb, 1)
}

func TestPreserveReplicaSnapshotsOwnWritesBeyondPrimary(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	r := &MariaDBReconciler{Client: fakeClient,
		replicaRecoveryGtidsFn: preserveTestGtids(preserveTestPrimaryState, "0-11-899,0-10-905", nil)}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreservePollDelay {
		t.Fatalf("expected the replica to be snapshotted, got %+v, %v", result, err)
	}
	expectPreserveSnapshots(t, r, mariadb, 1)
}

func TestPreserveReplicaRecoversReplicaHoldingNothingUnique(t *testing.T) {
	mariadb := preserveTestMariaDB(nil)
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	r := &MariaDBReconciler{Client: fakeClient, replicaRecoveryGtidsFn: preserveTestHealthyGtids()}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || !result.IsZero() {
		t.Fatalf("expected the recovery to go on, got %+v, %v", result, err)
	}
	expectPreserveSnapshots(t, r, mariadb, 0)
}

func TestPrimaryLacksReplicatedTransactions(t *testing.T) {
	tests := []struct {
		name         string
		replica      string
		primaryState string
		primarySlave string
		want         string
		wantSeq      uint64
		wantErr      bool
	}{
		{
			name:         "replica behind the primary",
			replica:      "0-11-850,0-10-400",
			primaryState: "0-11-900,0-10-400",
		},
		{
			name:    "primary recreated with empty storage",
			replica: "0-11-850,0-10-400",
			want:    "0-11-850",
			wantSeq: 0,
		},
		{
			name:         "primary recreated and wrote its own bootstrap transactions",
			replica:      "0-11-850",
			primaryState: "0-11-3",
			want:         "0-11-850",
			wantSeq:      3,
		},
		{
			name:         "primary promoted while behind the replica on the old primary stream",
			replica:      "0-12-905",
			primaryState: "0-11-900,0-12-880",
			want:         "0-12-905",
			wantSeq:      900,
		},
		{
			name:         "own writes of the replica are left to the preserve checks",
			replica:      "0-11-899,0-10-905",
			primaryState: "0-11-900",
		},
		{
			name:         "primary holds a higher sequence of the domain through another server",
			replica:      "0-12-30",
			primarySlave: "0-11-500",
		},
		{
			name:         "domain unknown to the primary",
			replica:      "0-11-850,1-11-20",
			primaryState: "0-11-900",
			want:         "1-11-20",
			wantSeq:      0,
		},
		{
			name:    "replica holding nothing",
			replica: "",
		},
		{
			name:    "malformed replica GTID is an error",
			replica: "0-11",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, seq, err := primaryLacksReplicatedTransactions(preserveTestReplicaServerID, tt.replica,
				tt.primaryState, tt.primarySlave)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := ""
			if missing != nil {
				got = missing.String()
			}
			if got != tt.want || seq != tt.wantSeq {
				t.Fatalf("unexpected result: got %q at primary sequence %d, want %q at %d", got, seq, tt.want, tt.wantSeq)
			}
		})
	}
}

const (
	preserveTestReplicaServerID = 10
	preserveTestPrimaryState    = "0-10-400,0-11-900"
	preserveTestReplicaState    = "0-10-400,0-11-850"
)

func preserveTestGtids(primaryBinlogState, replicaBinlogState string,
	replicaErr error) func(context.Context, *mariadbv1alpha1.MariaDB, int) (*replicaRecoveryGtids, error) {
	return func(context.Context, *mariadbv1alpha1.MariaDB, int) (*replicaRecoveryGtids, error) {
		gtids := &replicaRecoveryGtids{
			PrimaryBinlogState: primaryBinlogState,
			ReplicaErr:         replicaErr,
		}
		if replicaErr == nil {
			gtids.ReplicaServerID = preserveTestReplicaServerID
			gtids.ReplicaBinlogState = replicaBinlogState
			gtids.ReplicaBinlogPos = replicaBinlogState
		}
		return gtids, nil
	}
}

func preserveTestHealthyGtids() func(context.Context, *mariadbv1alpha1.MariaDB, int) (*replicaRecoveryGtids, error) {
	return preserveTestGtids(preserveTestPrimaryState, preserveTestReplicaState, nil)
}

func preserveTestPrimaryPVC(uid types.UID) *corev1.PersistentVolumeClaim {
	mariadb := preserveTestMariaDB(nil)
	key := mariadb.PVCKey(builder.StorageVolume, 1)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			UID:       uid,
		},
	}
}

func expectPrimaryRefusal(t *testing.T, r *MariaDBReconciler, mariadb *mariadbv1alpha1.MariaDB,
	recorder *events.FakeRecorder, reason string) {
	t.Helper()
	event := <-recorder.Events
	if !strings.Contains(event, mariadbv1alpha1.ReasonMariaDBReplicaRecoveryRefused) || !strings.Contains(event, reason) ||
		!strings.Contains(event, "A preserve snapshot does not lift this") {
		t.Fatalf("unexpected event: %s", event)
	}
	expectPreserveSnapshots(t, r, mariadb, 0)
}

func expectPreserveSnapshots(t *testing.T, r *MariaDBReconciler, mariadb *mariadbv1alpha1.MariaDB, want int) {
	t.Helper()
	snapshots, err := r.listReplicaRecoveryPreserveSnapshots(context.Background(), mariadb, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != want {
		t.Fatalf("expected %d preserve snapshots, got %d", want, len(snapshots))
	}
}

func preserveTestReady() *volumesnapshotv1.VolumeSnapshotStatus {
	return &volumesnapshotv1.VolumeSnapshotStatus{ReadyToUse: ptr.To(true)}
}

func TestReplicaRecoveryPreserveEpisode(t *testing.T) {
	recovering := preserveTestMariaDB(nil)
	recovered := preserveTestMariaDB(nil)
	recovered.Status.Conditions = nil
	pvc := preserveTestPVC(recovering, "replica-uid")

	if got := replicaRecoveryPreserveEpisode(recovering, pvc); got != preserveTestRecoveryID {
		t.Fatalf("unexpected episode while recovering: %s", got)
	}
	if got := replicaRecoveryPreserveEpisode(recovered, pvc); got != "replica-uid" {
		t.Fatalf("unexpected episode without a recovery condition: %s", got)
	}
}

func TestPreserveReplicaSkipsRecordedEpisode(t *testing.T) {
	mariadb := preserveTestMariaDB(nil)
	mariadb.Annotations[replicaRecoveryPreservedAnnotationKey(0)] = preserveTestRecoveryID
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	r := &MariaDBReconciler{Client: fakeClient}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || !result.IsZero() {
		t.Fatalf("expected the verified episode to go on without the replica Pod, got %+v, %v", result, err)
	}
}

func TestPreserveReplicaRechecksEarlierEpisode(t *testing.T) {
	mariadb := preserveTestMariaDB(nil)
	mariadb.Annotations[replicaRecoveryPreservedAnnotationKey(0)] = "replica-uid-1"
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid")).Build()
	r := &MariaDBReconciler{Client: fakeClient, RefResolver: refresolver.New(fakeClient)}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || result.RequeueAfter != replicaRecoveryPreserveRefusedDelay {
		t.Fatalf("expected an earlier recovery not to count, got %+v, %v", result, err)
	}
}

func TestPreserveReplicaRecordsReadySnapshot(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(mariadb, preserveTestPVC(mariadb, "replica-uid"),
			preserveTestSnapshot(mariadb, "db-cluster-preserve-0-a", preserveTestRecoveryID, preserveTestBase, preserveTestReady())).Build()
	r := &MariaDBReconciler{Client: fakeClient, replicaRecoveryGtidsFn: preserveTestHealthyGtids()}

	result, err := r.preserveReplicaBeforeRecovery(context.Background(), mariadb, "db-cluster-0", logr.Discard())
	if err != nil || !result.IsZero() {
		t.Fatalf("expected the recovery to go on, got %+v, %v", result, err)
	}
	var stored mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &stored); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stored.Annotations[replicaRecoveryPreservedAnnotationKey(0)] != preserveTestRecoveryID {
		t.Fatalf("expected the preserved episode to be recorded, got %v", stored.Annotations)
	}
}

func TestExpireReplicaRecoveryPreserveSnapshots(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	other := preserveTestMariaDB(ptr.To(preserveTestClass))
	other.Name = "other-cluster"
	day := 24 * time.Hour
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(
			preserveTestSnapshot(mariadb, "db-cluster-preserve-0-8d", "recovery-a", preserveTestBase.Add(-8*day), preserveTestReady()),
			preserveTestSnapshot(mariadb, "db-cluster-preserve-0-7d", "recovery-b", preserveTestBase.Add(-7*day), preserveTestReady()),
			preserveTestSnapshot(mariadb, "db-cluster-preserve-0-6d", "recovery-c", preserveTestBase.Add(-6*day), preserveTestReady()),
			preserveTestSnapshot(other, "other-cluster-preserve-0-30d", "recovery-d", preserveTestBase.Add(-30*day), preserveTestReady()),
		).Build()
	r := &MariaDBReconciler{Client: fakeClient}

	next, err := r.expireReplicaRecoveryPreserveSnapshots(context.Background(), mariadb, preserveTestBase, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != day {
		t.Fatalf("expected the next expiry in one day, got %v", next)
	}
	var list volumesnapshotv1.VolumeSnapshotList
	if err := fakeClient.List(context.Background(), &list); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := map[string]bool{}
	for _, s := range list.Items {
		names[s.Name] = true
	}
	if len(names) != 2 || !names["db-cluster-preserve-0-6d"] || !names["other-cluster-preserve-0-30d"] {
		t.Fatalf("unexpected snapshots after expiry: %v", names)
	}
}

func TestExpireReplicaRecoveryPreserveSnapshotsDeletesNewest(t *testing.T) {
	mariadb := preserveTestMariaDB(ptr.To(preserveTestClass))
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithObjects(preserveTestSnapshot(mariadb, "db-cluster-preserve-0-only", preserveTestRecoveryID,
			preserveTestBase.Add(-replicaRecoveryPreserveMaxAge-time.Minute), preserveTestReady())).Build()
	r := &MariaDBReconciler{Client: fakeClient}

	next, err := r.expireReplicaRecoveryPreserveSnapshots(context.Background(), mariadb, preserveTestBase, logr.Discard())
	if err != nil || next != 0 {
		t.Fatalf("expected nothing left to expire, got %v, %v", next, err)
	}
	snapshots, err := r.listReplicaRecoveryPreserveSnapshots(context.Background(), mariadb, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expected the only snapshot to expire, got %d", len(snapshots))
	}
}

func TestExpireReplicaRecoveryPreserveSnapshotsWithoutSnapshotAPI(t *testing.T) {
	mariadb := preserveTestMariaDB(nil)
	fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
				return &meta.NoKindMatchError{GroupKind: volumesnapshotv1.SchemeGroupVersion.WithKind("VolumeSnapshot").GroupKind()}
			},
		}).Build()
	r := &MariaDBReconciler{Client: fakeClient}

	next, err := r.expireReplicaRecoveryPreserveSnapshots(context.Background(), mariadb, preserveTestBase, logr.Discard())
	if err != nil || next != 0 {
		t.Fatalf("expected no expiry without the VolumeSnapshot API, got %v, %v", next, err)
	}
}
