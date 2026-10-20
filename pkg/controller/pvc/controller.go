package pvc

import (
	"context"
	"maps"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PVCReconciler struct {
	client.Client
}

func NewPVCReconciler(client client.Client) *PVCReconciler {
	return &PVCReconciler{
		Client: client,
	}
}

func (r *PVCReconciler) Reconcile(ctx context.Context, key types.NamespacedName, pvc *corev1.PersistentVolumeClaim) error {
	var existingPVC corev1.PersistentVolumeClaim
	err := r.Get(ctx, key, &existingPVC)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	return r.Create(ctx, pvc)
}

func (r *PVCReconciler) ReconcileMetadata(ctx context.Context, pvc *corev1.PersistentVolumeClaim,
	desired, previous metav1.ObjectMeta) error {
	original := pvc.DeepCopy()
	if _, tracked := pvc.Annotations[metadata.PVCLabelsAnnotation]; !tracked {
		// Adopt keys from the original StatefulSet template when upgrading existing claims.
		metadata.SetPVCMetadata(&pvc.ObjectMeta, previous)
	}
	metadata.SetPVCMetadata(&pvc.ObjectMeta, desired)
	if maps.Equal(original.Labels, pvc.Labels) && maps.Equal(original.Annotations, pvc.Annotations) {
		return nil
	}
	return r.Patch(ctx, pvc, client.MergeFrom(original))
}
