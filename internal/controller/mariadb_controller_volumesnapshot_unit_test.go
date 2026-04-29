package controller

import (
	"context"
	"testing"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetVolumeSnapshotKeyUsesCurrentPhysicalBackupRun(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := volumesnapshotv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding VolumeSnapshot scheme: %v", err)
	}

	createdAt := time.Unix(200, 0)
	physicalBackup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "mariadb-pb-scaleout",
			Namespace:         "test",
			CreationTimestamp: metav1.NewTime(createdAt),
		},
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			Storage: mariadbv1alpha1.PhysicalBackupStorage{
				VolumeSnapshot: &mariadbv1alpha1.PhysicalBackupVolumeSnapshot{
					VolumeSnapshotClassName: "csi-hostpath-snapclass",
				},
			},
		},
	}
	selectorLabels := labels.NewLabelsBuilder().
		WithPhysicalBackupSelectorLabels(physicalBackup).
		Build()
	oldSnapshot := &volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "old-snapshot",
			Namespace:         "test",
			CreationTimestamp: metav1.NewTime(createdAt.Add(-time.Minute)),
			Labels:            selectorLabels,
		},
	}
	currentSnapshot := &volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "current-snapshot",
			Namespace:         "test",
			CreationTimestamp: metav1.NewTime(createdAt.Add(time.Minute)),
			Labels:            selectorLabels,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(oldSnapshot, currentSnapshot).
		Build()
	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	key, err := reconciler.getVolumeSnapshotKey(context.Background(), &mariadbv1alpha1.MariaDB{}, physicalBackup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key == nil {
		t.Fatal("expected VolumeSnapshot key")
	}
	if key.Name != currentSnapshot.Name {
		t.Fatalf("expected newest snapshot from current run %q, got %q", currentSnapshot.Name, key.Name)
	}
}
