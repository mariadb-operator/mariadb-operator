package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/pvc"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func shouldReconcileStorage(mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb.IsRestoringBackup() || mdb.IsUpdating() || mdb.IsSwitchingPrimary() || mdb.HasGaleraNotReadyCondition() {
		return false
	}
	return true
}

func (r *MariaDBReconciler) reconcileStorage(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !shouldReconcileStorage(mariadb) {
		return ctrl.Result{}, nil
	}
	if mariadb.IsWaitingForStorageResize() {
		return r.waitForStorageResize(ctx, mariadb)
	}

	key := client.ObjectKeyFromObject(mariadb)
	var existingSts appsv1.StatefulSet
	if err := r.Get(ctx, key, &existingSts); err != nil {
		return ctrl.Result{}, err
	}

	existingSize := stsobj.GetStorageSize(&existingSts, builder.StorageVolume)
	desiredSize := mariadb.Spec.Storage.GetSize()
	if existingSize == nil {
		return ctrl.Result{}, errors.New("invalid existing storage size")
	}
	if desiredSize == nil {
		return ctrl.Result{}, errors.New("invalid desired storage size")
	}

	sizeCmp := desiredSize.Cmp(*existingSize)
	if sizeCmp == 0 {
		return ctrl.Result{}, nil
	}
	if sizeCmp < 0 {
		return ctrl.Result{}, fmt.Errorf("cannot decrease storage size from '%s' to '%s'", existingSize, desiredSize)
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReadyStorageResizing(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
	}

	if result, err := r.resizeInUsePVCs(ctx, mariadb, *desiredSize); !result.IsZero() || err != nil {
		return result, err
	}
	if result, err := r.resizeStatefulSet(ctx, mariadb, &existingSts); !result.IsZero() || err != nil {
		return result, err
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReadyWaitingStorageResize(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
	}

	return r.waitForStorageResize(ctx, mariadb)
}

func (r *MariaDBReconciler) resizeInUsePVCs(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	size resource.Quantity) (ctrl.Result, error) {
	if !ptr.Deref(mariadb.Spec.Storage.ResizeInUseVolumes, true) {
		return ctrl.Result{}, nil
	}

	pvcs, err := r.getStoragePVCs(ctx, mariadb)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, pvc := range pvcs {
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = size
		if err := r.Patch(ctx, &pvc, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching PVC '%s': %v", pvc.Name, err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) resizeStatefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	sts *appsv1.StatefulSet) (ctrl.Result, error) {
	if err := r.Delete(ctx, sts, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationOrphan)}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting StatefulSet: %v", err)
	}
	return r.reconcileStatefulSet(ctx, mariadb)
}

func (r *MariaDBReconciler) waitForStorageResize(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Waiting for storage resize")

	if ptr.Deref(mdb.Spec.Storage.ResizeInUseVolumes, true) && ptr.Deref(mdb.Spec.Storage.WaitForVolumeResize, true) {
		pvcs, err := r.getStoragePVCs(ctx, mdb)
		if err != nil {
			return ctrl.Result{}, err
		}
		for _, p := range pvcs {
			if pvc.IsResizing(&p) {
				logger.V(1).Info("Waiting for PVC resize", "pvc", p.Name)
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}
		}
	}

	key := client.ObjectKeyFromObject(mdb)
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, key, &sts); err != nil {
		return ctrl.Result{}, err
	}
	if sts.Status.ReadyReplicas != mdb.Spec.Replicas {
		logger.V(1).Info(
			"Waiting for StatefulSet ready",
			"ready-replicas", sts.Status.ReadyReplicas,
			"expected-replicas", mdb.Spec.Replicas,
		)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if err := r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReadyStorageResized(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) getStoragePVCs(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) ([]corev1.PersistentVolumeClaim, error) {
	pvcList := corev1.PersistentVolumeClaimList{}
	listOpts := client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mdb).
				WithPVCRole(builder.StorageVolumeRole).
				Build(),
		),
		Namespace: mdb.GetNamespace(),
	}
	if err := r.List(ctx, &pvcList, &listOpts); err != nil {
		return nil, fmt.Errorf("error listing PVCs: %v", err)
	}
	return pvcList.Items, nil
}
