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
	physicalBackup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			Storage: mariadbv1alpha1.PhysicalBackupStorage{
				PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{},
			},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}}
	r := &MariaDBReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(physicalBackup, pvc).Build()}

	if err := r.cleanupPhysicalBackup(context.Background(), key); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Get(context.Background(), key, &mariadbv1alpha1.PhysicalBackup{}); !apierrors.IsNotFound(err) {
		t.Errorf("expected PhysicalBackup to be deleted, got: %v", err)
	}
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{}); !apierrors.IsNotFound(err) {
		t.Errorf("expected storage PVC to be deleted, got: %v", err)
	}
}
