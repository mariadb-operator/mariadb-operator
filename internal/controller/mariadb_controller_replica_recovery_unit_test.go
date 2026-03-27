package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileReplicaRecoveryPreservesLostPVCRecoveryWhenRecoveryDisabled(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
			Annotations: map[string]string{
				storagePVCUIDAnnotationKey(0): "old-replica-uid",
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
				},
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(1),
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
			Name:      mariadb.PVCKey(builder.StorageVolume, 0).Name,
			Namespace: mariadb.Namespace,
			UID:       "new-replica-uid",
		},
	}
	primaryPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PVCKey(builder.StorageVolume, 1).Name,
			Namespace: mariadb.Namespace,
			UID:       "primary-uid",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, replicaPVC, primaryPVC).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.reconcileReplicaRecovery(context.Background(), mariadb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected requeue after 30s, got %v", result.RequeueAfter)
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if updated.Annotations[storagePVCUIDAnnotationKey(0)] != "old-replica-uid" {
		t.Fatalf("expected lost replica PVC annotation to be preserved until recovery")
	}
	if updated.ReplicaRecoveryError() == nil {
		t.Fatalf("expected replica recovery error to be set for lost PVC recovery without bootstrap source")
	}
}

func TestReconcileReplicaRecoveryDetectsFreshErroredReplicaAfterPVCUIDSync(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	creationTime := metav1.NewTime(time.Date(2026, 3, 13, 9, 0, 0, 0, time.UTC))
	replicaPVCBirth := metav1.NewTime(time.Date(2026, 3, 23, 21, 12, 55, 0, time.UTC))
	primaryPVCBirth := metav1.NewTime(time.Date(2026, 3, 13, 9, 19, 51, 0, time.UTC))

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "mariadb",
			Namespace:         "test",
			CreationTimestamp: creationTime,
			Annotations: map[string]string{
				storagePVCUIDAnnotationKey(0): "new-replica-uid",
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
				},
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(1),
			Replication: &mariadbv1alpha1.ReplicationStatus{
				Roles: map[string]mariadbv1alpha1.ReplicationRole{
					"mariadb-1": mariadbv1alpha1.ReplicationRolePrimary,
					"mariadb-0": mariadbv1alpha1.ReplicationRoleUnknown,
				},
				Replicas: map[string]mariadbv1alpha1.ReplicaStatus{
					"mariadb-0": {
						ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
							LastIOErrno:  ptr.To(0),
							LastSQLErrno: ptr.To(1396),
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
			UID:               "new-replica-uid",
			CreationTimestamp: replicaPVCBirth,
		},
	}
	primaryPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mariadb.PVCKey(builder.StorageVolume, 1).Name,
			Namespace:         mariadb.Namespace,
			UID:               "primary-uid",
			CreationTimestamp: primaryPVCBirth,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, replicaPVC, primaryPVC).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.reconcileReplicaRecovery(context.Background(), mariadb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected requeue after 30s, got %v", result.RequeueAfter)
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if updated.ReplicaRecoveryError() == nil {
		t.Fatalf("expected replica recovery error to be set for fresh errored PVC recovery without bootstrap source")
	}
}

func TestReconcileJobReplicaRecoveryKeepsReplicaPodDownWhileRecoveryJobRuns(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding apps scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding batch scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex: ptr.To(1),
					},
				},
			},
			Storage: mariadbv1alpha1.Storage{
				VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{},
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(1),
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
			Name:      mariadb.PVCKey(builder.StorageVolume, 0).Name,
			Namespace: mariadb.Namespace,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.Name,
			Namespace: mariadb.Namespace,
		},
	}
	replicaPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-0",
			Namespace: mariadb.Namespace,
		},
		Spec: corev1.PodSpec{
			NodeName: "node-a",
		},
	}
	recoveryJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupInitJobKey(0).Name,
			Namespace: mariadb.Namespace,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, replicaPVC, sts, replicaPod, recoveryJob).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.reconcileJobReplicaRecovery(context.Background(), "mariadb-0", nil, mariadb, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("expected requeue after 1s, got %v", result.RequeueAfter)
	}

	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(sts), &appsv1.StatefulSet{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected StatefulSet to be deleted while recovery job runs, got err=%v", err)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(replicaPod), &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected replica Pod to be deleted while recovery job runs, got err=%v", err)
	}
}

func TestReconcileJobReplicaRecoveryDeletesStaleCompletedRecoveryJobOnRecreatedPVC(t *testing.T) {
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

	jobCreationTime := metav1.NewTime(time.Date(2026, 3, 23, 22, 34, 40, 0, time.UTC))
	pvcCreationTime := metav1.NewTime(time.Date(2026, 3, 23, 22, 36, 21, 0, time.UTC))

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex: ptr.To(1),
					},
				},
			},
			Storage: mariadbv1alpha1.Storage{
				VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{},
			},
		},
	}
	replicaPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mariadb.PVCKey(builder.StorageVolume, 0).Name,
			Namespace:         mariadb.Namespace,
			UID:               "new-pvc-uid",
			CreationTimestamp: pvcCreationTime,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
	replicaPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-0",
			Namespace: mariadb.Namespace,
		},
		Spec: corev1.PodSpec{
			NodeName: "node-a",
		},
	}
	staleRecoveryJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mariadb.PhysicalBackupInitJobKey(0).Name,
			Namespace:         mariadb.Namespace,
			CreationTimestamp: jobCreationTime,
		},
		Status: batchv1.JobStatus{
			CompletionTime: ptr.To(jobCreationTime),
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, replicaPVC, replicaPod, staleRecoveryJob).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.reconcileJobReplicaRecovery(context.Background(), "mariadb-0", nil, mariadb, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("expected requeue after 1s, got %v", result.RequeueAfter)
	}

	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(staleRecoveryJob), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected stale recovery Job to be deleted, got err=%v", err)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(replicaPod), &corev1.Pod{}); err != nil {
		t.Fatalf("expected replica Pod to remain until a fresh recovery Job is created, got err=%v", err)
	}
}

func TestEnsureReplicaPhysicalBackupCurrentDeletesStaleRecoveryBackup(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding batch scheme: %v", err)
	}

	recoveryStart := metav1.NewTime(time.Date(2026, 3, 23, 22, 36, 21, 0, time.UTC))
	backupTime := metav1.NewTime(time.Date(2026, 3, 23, 22, 33, 44, 0, time.UTC))

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:               mariadbv1alpha1.ConditionTypeReplicaRecovered,
					Status:             metav1.ConditionFalse,
					Reason:             mariadbv1alpha1.ConditionReasonReplicaRecovered,
					LastTransitionTime: recoveryStart,
				},
			},
		},
	}
	physicalBackup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mariadb.PhysicalBackupReplicaRecoveryKey().Name,
			Namespace:         mariadb.Namespace,
			CreationTimestamp: backupTime,
		},
	}
	initJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupInitJobKey(0).Name,
			Namespace: mariadb.Namespace,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, physicalBackup, initJob).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.ensureReplicaPhysicalBackupCurrent(
		context.Background(),
		mariadb.PhysicalBackupReplicaRecoveryKey(),
		mariadb,
		nil,
		nil,
		logr.Discard(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("expected requeue after 1s, got %v", result.RequeueAfter)
	}

	err = fakeClient.Get(
		context.Background(),
		client.ObjectKeyFromObject(physicalBackup),
		&mariadbv1alpha1.PhysicalBackup{},
	)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected stale PhysicalBackup to be deleted, got err=%v", err)
	}
	err = fakeClient.Get(
		context.Background(),
		client.ObjectKeyFromObject(initJob),
		&batchv1.Job{},
	)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected stale init Job to be deleted, got err=%v", err)
	}
}

func TestEnsureReplicaPhysicalBackupCurrentDeletesStaleRecoveryArtifactsForRecreatedPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding batch scheme: %v", err)
	}

	backupTime := metav1.NewTime(time.Date(2026, 3, 23, 22, 33, 44, 0, time.UTC))
	pvcCreationTime := metav1.NewTime(time.Date(2026, 3, 23, 22, 36, 21, 0, time.UTC))

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
		},
	}
	physicalBackup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mariadb.PhysicalBackupReplicaRecoveryKey().Name,
			Namespace:         mariadb.Namespace,
			CreationTimestamp: backupTime,
		},
	}
	initJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupInitJobKey(0).Name,
			Namespace: mariadb.Namespace,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, physicalBackup, initJob).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.ensureReplicaPhysicalBackupCurrent(
		context.Background(),
		mariadb.PhysicalBackupReplicaRecoveryKey(),
		mariadb,
		map[int]storagePVCState{
			0: {
				UID:               "new-replica-pvc-uid",
				CreationTimestamp: pvcCreationTime,
			},
		},
		[]string{"mariadb-0"},
		logr.Discard(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("expected requeue after 1s, got %v", result.RequeueAfter)
	}

	err = fakeClient.Get(
		context.Background(),
		client.ObjectKeyFromObject(physicalBackup),
		&mariadbv1alpha1.PhysicalBackup{},
	)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected stale PhysicalBackup to be deleted, got err=%v", err)
	}
	err = fakeClient.Get(
		context.Background(),
		client.ObjectKeyFromObject(initJob),
		&batchv1.Job{},
	)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected stale init Job to be deleted, got err=%v", err)
	}
}

func TestEnsureReplicaPhysicalBackupCurrentKeepsCurrentRecoveryArtifactsForPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding batch scheme: %v", err)
	}

	backupTime := metav1.NewTime(time.Date(2026, 3, 23, 22, 36, 21, 0, time.UTC))
	pvcCreationTime := metav1.NewTime(time.Date(2026, 3, 23, 22, 33, 44, 0, time.UTC))

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
		},
	}
	physicalBackup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mariadb.PhysicalBackupReplicaRecoveryKey().Name,
			Namespace:         mariadb.Namespace,
			CreationTimestamp: backupTime,
		},
	}
	initJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupInitJobKey(0).Name,
			Namespace: mariadb.Namespace,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, physicalBackup, initJob).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.ensureReplicaPhysicalBackupCurrent(
		context.Background(),
		mariadb.PhysicalBackupReplicaRecoveryKey(),
		mariadb,
		map[int]storagePVCState{
			0: {
				UID:               "current-replica-pvc-uid",
				CreationTimestamp: pvcCreationTime,
			},
		},
		[]string{"mariadb-0"},
		logr.Discard(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("expected no requeue, got %+v", result)
	}

	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(physicalBackup), &mariadbv1alpha1.PhysicalBackup{}); err != nil {
		t.Fatalf("expected current PhysicalBackup to remain, got err=%v", err)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(initJob), &batchv1.Job{}); err != nil {
		t.Fatalf("expected current init Job to remain, got err=%v", err)
	}
}

func TestResetReplicaRecoveryIfNotNeededCleansRecoveryArtifacts(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding batch scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
			Annotations: map[string]string{
				storagePVCUIDAnnotationKey(0):                "replica-uid",
				storagePVCUIDAnnotationKey(1):                "primary-uid",
				replicaRecoveryRefreshPVCUIDAnnotationKey(0): "replica-uid",
				replicaRecoveryRefreshPVCUIDAnnotationKey(1): "primary-uid",
			},
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:   mariadbv1alpha1.ConditionTypeReplicaRecovered,
					Status: metav1.ConditionFalse,
					Reason: mariadbv1alpha1.ConditionReasonReplicaRecovered,
				},
			},
		},
	}
	physicalBackup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupReplicaRecoveryKey().Name,
			Namespace: mariadb.Namespace,
		},
	}
	initJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupInitJobKey(0).Name,
			Namespace: mariadb.Namespace,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, physicalBackup, initJob).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	handled, err := reconciler.resetReplicaRecoveryIfNotNeeded(
		context.Background(),
		mariadb,
		false,
		nil,
		map[int]string{
			0: "replica-uid",
			1: "primary-uid",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected replica recovery reset to handle cleanup")
	}

	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(physicalBackup), &mariadbv1alpha1.PhysicalBackup{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected replica recovery PhysicalBackup to be deleted, got err=%v", err)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(initJob), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected replica recovery init Job to be deleted, got err=%v", err)
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if _, ok := updated.Annotations[replicaRecoveryRefreshPVCUIDAnnotationKey(0)]; ok {
		t.Fatalf("expected replica recovery retry annotation to be cleared")
	}
	if updated.IsRecoveringReplicas() {
		t.Fatalf("expected replica recovery condition to be reset")
	}
}

func TestRetryReplicaRecoveryWithFreshBackupDeletesArtifactsOncePerPVC(t *testing.T) {
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
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
		},
	}
	replicaPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PVCKey(builder.StorageVolume, 0).Name,
			Namespace: mariadb.Namespace,
			UID:       "retry-pvc-uid",
		},
	}
	physicalBackup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupReplicaRecoveryKey().Name,
			Namespace: mariadb.Namespace,
		},
	}
	initJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupInitJobKey(0).Name,
			Namespace: mariadb.Namespace,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, replicaPVC, physicalBackup, initJob).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	retried, err := reconciler.retryReplicaRecoveryWithFreshBackup(
		context.Background(),
		"mariadb-0",
		mariadb,
		context.DeadlineExceeded,
		logr.Discard(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !retried {
		t.Fatalf("expected replica recovery to schedule a fresh-backup retry")
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if got := updated.Annotations[replicaRecoveryRefreshPVCUIDAnnotationKey(0)]; got != "retry-pvc-uid" {
		t.Fatalf("expected retry annotation to be set, got %q", got)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(physicalBackup), &mariadbv1alpha1.PhysicalBackup{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected replica recovery PhysicalBackup to be deleted, got err=%v", err)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(initJob), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected replica recovery init Job to be deleted, got err=%v", err)
	}

	retryBackup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupReplicaRecoveryKey().Name,
			Namespace: mariadb.Namespace,
		},
	}
	retryJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupInitJobKey(0).Name,
			Namespace: mariadb.Namespace,
		},
	}
	if err := fakeClient.Create(context.Background(), retryBackup); err != nil {
		t.Fatalf("error creating retry PhysicalBackup: %v", err)
	}
	if err := fakeClient.Create(context.Background(), retryJob); err != nil {
		t.Fatalf("error creating retry Job: %v", err)
	}

	retried, err = reconciler.retryReplicaRecoveryWithFreshBackup(
		context.Background(),
		"mariadb-0",
		&updated,
		context.DeadlineExceeded,
		logr.Discard(),
	)
	if err != nil {
		t.Fatalf("unexpected error on second retry attempt: %v", err)
	}
	if retried {
		t.Fatalf("expected only one fresh-backup retry per PVC")
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(retryBackup), &mariadbv1alpha1.PhysicalBackup{}); err != nil {
		t.Fatalf("expected PhysicalBackup to remain after retry budget is exhausted, got err=%v", err)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(retryJob), &batchv1.Job{}); err != nil {
		t.Fatalf("expected init Job to remain after retry budget is exhausted, got err=%v", err)
	}
}

func TestRetryReplicaRecoveryWithFreshBackupIgnoresNonTimeoutErrors(t *testing.T) {
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
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
		},
	}
	replicaPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PVCKey(builder.StorageVolume, 0).Name,
			Namespace: mariadb.Namespace,
			UID:       "retry-pvc-uid",
		},
	}
	physicalBackup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupReplicaRecoveryKey().Name,
			Namespace: mariadb.Namespace,
		},
	}
	initJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupInitJobKey(0).Name,
			Namespace: mariadb.Namespace,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, replicaPVC, physicalBackup, initJob).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	retried, err := reconciler.retryReplicaRecoveryWithFreshBackup(
		context.Background(),
		"mariadb-0",
		mariadb,
		errors.New("access denied"),
		logr.Discard(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retried {
		t.Fatalf("expected non-timeout replication errors to skip fresh-backup retry")
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if _, ok := updated.Annotations[replicaRecoveryRefreshPVCUIDAnnotationKey(0)]; ok {
		t.Fatalf("expected retry annotation to remain unset for non-timeout errors")
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(physicalBackup), &mariadbv1alpha1.PhysicalBackup{}); err != nil {
		t.Fatalf("expected PhysicalBackup to remain for non-timeout errors, got err=%v", err)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(initJob), &batchv1.Job{}); err != nil {
		t.Fatalf("expected init Job to remain for non-timeout errors, got err=%v", err)
	}
}
