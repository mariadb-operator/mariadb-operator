package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	corev1 "k8s.io/api/core/v1"
)

func TestSyncStoragePVCUIDAnnotationsRefreshesManagedSQLResourcesOnPrimaryChange(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
			Annotations: map[string]string{
				storagePVCUIDAnnotationKey(0): "old-uid",
			},
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 1,
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(0),
		},
	}
	database := &mariadbv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app",
			Namespace: mariadb.Namespace,
		},
		Spec: mariadbv1alpha1.DatabaseSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name: mariadb.Name,
				},
			},
		},
	}
	user := &mariadbv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-user",
			Namespace: mariadb.Namespace,
		},
		Spec: mariadbv1alpha1.UserSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name: mariadb.Name,
				},
			},
		},
	}
	grant := &mariadbv1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-grant",
			Namespace: mariadb.Namespace,
		},
		Spec: mariadbv1alpha1.GrantSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name: mariadb.Name,
				},
			},
			Privileges: []string{"SELECT"},
			Database:   "*",
			Table:      "*",
			Username:   "app-user",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, database, user, grant).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	if err := reconciler.syncStoragePVCUIDAnnotations(context.Background(), mariadb, map[int]string{0: "new-uid"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertObjectAnnotation(
		t,
		fakeClient,
		client.ObjectKeyFromObject(database),
		&mariadbv1alpha1.Database{},
		sqlReconcileTokenAnnotation,
		"new-uid",
	)
	assertObjectAnnotation(
		t,
		fakeClient,
		client.ObjectKeyFromObject(user),
		&mariadbv1alpha1.User{},
		sqlReconcileTokenAnnotation,
		"new-uid",
	)
	assertObjectAnnotation(
		t,
		fakeClient,
		client.ObjectKeyFromObject(grant),
		&mariadbv1alpha1.Grant{},
		sqlReconcileTokenAnnotation,
		"new-uid",
	)
	assertObjectAnnotation(
		t,
		fakeClient,
		client.ObjectKeyFromObject(mariadb),
		&mariadbv1alpha1.MariaDB{},
		storagePVCUIDAnnotationKey(0),
		"new-uid",
	)
}

func TestReconcilePrimaryPVCFailoverPromotesReplicaOnPrimaryPVCChange(t *testing.T) {
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
				storagePVCUIDAnnotationKey(0): "old-primary-uid",
				storagePVCUIDAnnotationKey(1): "replica-uid",
			},
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex: ptr.To(0),
					},
				},
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(0),
		},
	}
	primaryPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PVCKey(builder.StorageVolume, 0).Name,
			Namespace: mariadb.Namespace,
			UID:       "new-primary-uid",
		},
	}
	replicaPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PVCKey(builder.StorageVolume, 1).Name,
			Namespace: mariadb.Namespace,
			UID:       "replica-uid",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, primaryPVC, replicaPVC).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
		FailoverCandidateFn: func(context.Context, *mariadbv1alpha1.MariaDB, logr.Logger) (string, error) {
			return "mariadb-1", nil
		},
	}

	result, err := reconciler.reconcilePrimaryPVCFailover(context.Background(), mariadb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsZero() {
		t.Fatalf("expected reconcile to requeue after triggering failover")
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if got := ptr.Deref(updated.Spec.Replication.Primary.PodIndex, -1); got != 1 {
		t.Fatalf("expected spec primary pod index 1, got %d", got)
	}
	if got := ptr.Deref(updated.Status.CurrentPrimaryPodIndex, -1); got != 1 {
		t.Fatalf("expected status primary pod index 1, got %d", got)
	}
}

func TestReconcilePrimaryPVCFailoverPromotesExternallyPromotedPrimaryOnPrimaryPVCChange(t *testing.T) {
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
				storagePVCUIDAnnotationKey(0): "old-primary-uid",
				storagePVCUIDAnnotationKey(1): "replica-uid",
			},
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex: ptr.To(0),
					},
				},
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(0),
		},
	}
	primaryPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PVCKey(builder.StorageVolume, 0).Name,
			Namespace: mariadb.Namespace,
			UID:       "new-primary-uid",
		},
	}
	replicaPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PVCKey(builder.StorageVolume, 1).Name,
			Namespace: mariadb.Namespace,
			UID:       "replica-uid",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, primaryPVC, replicaPVC).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
		FailoverCandidateFn: func(context.Context, *mariadbv1alpha1.MariaDB, logr.Logger) (string, error) {
			return "", errors.New("no promotion candidates were found")
		},
		PromotedPrimaryCandidateFn: func(context.Context, *mariadbv1alpha1.MariaDB, logr.Logger) (string, error) {
			return "mariadb-1", nil
		},
	}

	result, err := reconciler.reconcilePrimaryPVCFailover(context.Background(), mariadb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsZero() {
		t.Fatalf("expected reconcile to requeue after promoting externally promoted primary")
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if got := ptr.Deref(updated.Spec.Replication.Primary.PodIndex, -1); got != 1 {
		t.Fatalf("expected spec primary pod index 1, got %d", got)
	}
	if got := ptr.Deref(updated.Status.CurrentPrimaryPodIndex, -1); got != 1 {
		t.Fatalf("expected status primary pod index 1, got %d", got)
	}
}

func TestReconcilePrimaryPVCFailoverPromotesReusableReplicaOnInitialBootstrap(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	creationTime := metav1.NewTime(time.Date(2026, 3, 25, 1, 0, 0, 0, time.UTC))
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "mariadb",
			Namespace:         "test",
			CreationTimestamp: creationTime,
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 3,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex: ptr.To(0),
					},
				},
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(0),
		},
	}
	freshPrimaryPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mariadb.PVCKey(builder.StorageVolume, 0).Name,
			Namespace:         mariadb.Namespace,
			UID:               "fresh-primary-uid",
			CreationTimestamp: metav1.NewTime(creationTime.Add(5 * time.Minute)),
		},
	}
	reusableReplicaPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mariadb.PVCKey(builder.StorageVolume, 1).Name,
			Namespace:         mariadb.Namespace,
			UID:               "reusable-replica-uid",
			CreationTimestamp: metav1.NewTime(creationTime.Add(-5 * time.Minute)),
		},
	}
	otherReusableReplicaPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mariadb.PVCKey(builder.StorageVolume, 2).Name,
			Namespace:         mariadb.Namespace,
			UID:               "other-reusable-replica-uid",
			CreationTimestamp: metav1.NewTime(creationTime.Add(-4 * time.Minute)),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, freshPrimaryPVC, reusableReplicaPVC, otherReusableReplicaPVC).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.reconcilePrimaryPVCFailover(context.Background(), mariadb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsZero() {
		t.Fatalf("expected reconcile to requeue after promoting reusable replica")
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if got := ptr.Deref(updated.Spec.Replication.Primary.PodIndex, -1); got != 1 {
		t.Fatalf("expected spec primary pod index 1, got %d", got)
	}
	if got := ptr.Deref(updated.Status.CurrentPrimaryPodIndex, -1); got != 1 {
		t.Fatalf("expected status primary pod index 1, got %d", got)
	}
}

func TestGetPrimaryPVCChange(t *testing.T) {
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				storagePVCUIDAnnotationKey(0): "primary-uid",
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(0),
		},
	}

	change, ok := getPrimaryPVCChange(mariadb, map[int]string{0: "new-primary-uid"})
	if !ok {
		t.Fatalf("expected primary PVC change to be detected")
	}
	if change.PodIndex != 0 || change.StoredUID != "primary-uid" || change.CurrentUID != "new-primary-uid" {
		t.Fatalf("unexpected primary PVC change: %#v", change)
	}

	if _, ok := getPrimaryPVCChange(mariadb, map[int]string{0: "primary-uid"}); ok {
		t.Fatalf("expected unchanged primary PVC to be ignored")
	}
}

func TestGetInitialPrimaryPVCBootstrapCandidate(t *testing.T) {
	creationTime := metav1.NewTime(time.Date(2026, 3, 25, 1, 0, 0, 0, time.UTC))

	testCases := map[string]struct {
		mariadb   *mariadbv1alpha1.MariaDB
		pvcStates map[int]storagePVCState
		want      *int
	}{
		"reuses older replica when primary pvc is fresh": {
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: creationTime,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			pvcStates: map[int]storagePVCState{
				0: {
					UID:               "fresh-primary-uid",
					CreationTimestamp: metav1.NewTime(creationTime.Add(5 * time.Minute)),
				},
				1: {
					UID:               "reusable-replica-uid",
					CreationTimestamp: metav1.NewTime(creationTime.Add(-5 * time.Minute)),
				},
			},
			want: ptr.To(1),
		},
		"does not infer candidate when primary pvc is reusable": {
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: creationTime,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 2,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			pvcStates: map[int]storagePVCState{
				0: {
					UID:               "primary-uid",
					CreationTimestamp: metav1.NewTime(creationTime.Add(-5 * time.Minute)),
				},
				1: {
					UID:               "replica-uid",
					CreationTimestamp: metav1.NewTime(creationTime.Add(-4 * time.Minute)),
				},
			},
		},
		"does not infer candidate when pvc annotations are already tracked": {
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: creationTime,
					Annotations: map[string]string{
						storagePVCUIDAnnotationKey(0): "tracked-primary-uid",
					},
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 2,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			pvcStates: map[int]storagePVCState{
				0: {
					UID:               "fresh-primary-uid",
					CreationTimestamp: metav1.NewTime(creationTime.Add(5 * time.Minute)),
				},
				1: {
					UID:               "reusable-replica-uid",
					CreationTimestamp: metav1.NewTime(creationTime.Add(-5 * time.Minute)),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := getInitialPrimaryPVCBootstrapCandidate(tc.mariadb, tc.pvcStates)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected no bootstrap candidate, got %d", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected bootstrap candidate %d, got nil", *tc.want)
			}
			if *got != *tc.want {
				t.Fatalf("expected bootstrap candidate %d, got %d", *tc.want, *got)
			}
		})
	}
}

func assertObjectAnnotation(t *testing.T, c client.Client, key client.ObjectKey, obj client.Object, annotationKey, expected string) {
	t.Helper()
	if err := c.Get(context.Background(), key, obj); err != nil {
		t.Fatalf("error getting object %s: %v", key.Name, err)
	}
	if got := obj.GetAnnotations()[annotationKey]; got != expected {
		t.Fatalf("expected annotation %s=%s on %s, got %q", annotationKey, expected, key.Name, got)
	}
}
