package statefulset

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatefulSetReconciler struct {
	client.Client
}

type StatefulSetUpdateFn func(existingSts, desiredSts *appsv1.StatefulSet) (bool, error)

func NewStatefulSetReconciler(client client.Client) *StatefulSetReconciler {
	return &StatefulSetReconciler{
		Client: client,
	}
}

func (r *StatefulSetReconciler) ReconcileWithUpdateFn(ctx context.Context, desiredSts *appsv1.StatefulSet,
	shouldUpdateFn StatefulSetUpdateFn) error {

	key := client.ObjectKeyFromObject(desiredSts)
	var existingSts appsv1.StatefulSet
	if err := r.Get(ctx, key, &existingSts); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting StatefulSet: %v", err)
		}
		if err := r.Create(ctx, desiredSts); err != nil {
			return fmt.Errorf("error creating StatefulSet: %v", err)
		}
		return nil
	}

	shouldUpdate, err := shouldUpdateFn(&existingSts, desiredSts)
	if err != nil {
		return fmt.Errorf("error checking StatefulSet update: %v", err)
	}
	if !shouldUpdate || !StatefulSetHasChanged(&existingSts, desiredSts) {
		return nil
	}

	patch := client.MergeFrom(existingSts.DeepCopy())
	existingSts.Spec.Template = desiredSts.Spec.Template
	existingSts.Spec.UpdateStrategy = desiredSts.Spec.UpdateStrategy
	existingSts.Spec.Replicas = desiredSts.Spec.Replicas
	return r.Patch(ctx, &existingSts, patch)
}

func (r *StatefulSetReconciler) Reconcile(ctx context.Context, desiredSts *appsv1.StatefulSet) error {
	return r.ReconcileWithUpdateFn(ctx, desiredSts, func(existingSts, desiredSts *appsv1.StatefulSet) (bool, error) {
		return true, nil
	})
}

func StatefulSetHasChanged(existingSts, desiredSts *appsv1.StatefulSet) bool {
	return existingSts == nil || desiredSts == nil ||
		!reflect.DeepEqual(existingSts.Spec.Template, desiredSts.Spec.Template) ||
		!reflect.DeepEqual(existingSts.Spec.UpdateStrategy, desiredSts.Spec.UpdateStrategy) ||
		ptr.Deref(existingSts.Spec.Replicas, int32(0)) != ptr.Deref(desiredSts.Spec.Replicas, int32(0))
}
