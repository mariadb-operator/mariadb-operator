package pvc

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
