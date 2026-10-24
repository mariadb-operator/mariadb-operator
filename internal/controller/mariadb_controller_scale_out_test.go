package controller

import (
	"context"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCleanupPhysicalBackupDeletesStoragePVC(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mdb := &mariadbv1alpha1.MariaDB{ObjectMeta: metav1.ObjectMeta{Name: "mariadb", Namespace: "default"}}
	key := mdb.PhysicalBackupReplicaRecoveryKey()

	tests := []struct {
		name         string
		storage      mariadbv1alpha1.PhysicalBackupStorage
		wantPVCExist bool
	}{
		{
			name: "persistentVolumeClaim storage PVC is deleted",
			storage: mariadbv1alpha1.PhysicalBackupStorage{
				PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{},
			},
			wantPVCExist: false,
		},
		{
			// The user-provided PVC has the same name as the PhysicalBackup on purpose,
			// to assert that only the storage type decides whether the PVC is deleted.
			name: "volume persistentVolumeClaim PVC is not deleted",
			storage: mariadbv1alpha1.PhysicalBackupStorage{
				Volume: &mariadbv1alpha1.StorageVolumeSource{
					PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{
						ClaimName: key.Name,
					},
				},
			},
			wantPVCExist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			physicalBackup := &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: tt.storage,
				},
			}
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(physicalBackup, pvc).Build()
			r := &MariaDBReconciler{Client: fakeClient}

			if err := r.cleanupPhysicalBackup(context.Background(), key); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := r.Get(testCtx, key, &mariadbv1alpha1.PhysicalBackup{}); !apierrors.IsNotFound(err) {
				t.Errorf("expected PhysicalBackup to be deleted, got: %v", err)
			}
			err := r.Get(testCtx, client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{})
			if tt.wantPVCExist && err != nil {
				t.Errorf("expected PVC to be kept, got: %v", err)
			}
			if !tt.wantPVCExist && !apierrors.IsNotFound(err) {
				t.Errorf("expected storage PVC to be deleted, got: %v", err)
			}
		})
	}
}
