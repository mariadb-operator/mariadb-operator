package controller

import (
	"context"
	"fmt"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func (r *PhysicalBackupReconciler) reconcileVolumeSnapshots(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	_, err := r.listVolumeSnapshots(ctx, backup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing VolumeSnapshots: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *PhysicalBackupReconciler) listVolumeSnapshots(ctx context.Context,
	backup *mariadbv1alpha1.PhysicalBackup) (*volumesnapshotv1.VolumeSnapshotList, error) {
	var volumeSnapshotList volumesnapshotv1.VolumeSnapshotList
	if err := r.List(
		ctx,
		&volumeSnapshotList,
		client.InNamespace(backup.Namespace),
		client.MatchingFields{metaCtrlFieldPath: backup.Name},
	); err != nil {
		return nil, err
	}
	return &volumeSnapshotList, nil
}

func (r *PhysicalBackupReconciler) indexVolumeSnapshots(ctx context.Context, mgr manager.Manager) error {
	log.FromContext(ctx).
		WithName("indexer").
		WithValues(
			"kind", "VolumeSnapshot",
			"field", metaCtrlFieldPath,
		).
		Info("Watching field")
	return mgr.GetFieldIndexer().IndexField(
		ctx,
		&volumesnapshotv1.VolumeSnapshot{},
		metaCtrlFieldPath,
		func(o client.Object) []string {
			volumeSnapshot, ok := o.(*volumesnapshotv1.VolumeSnapshot)
			if !ok {
				return nil
			}
			owner := metav1.GetControllerOf(volumeSnapshot)
			if owner == nil {
				return nil
			}
			if owner.Kind != mariadbv1alpha1.PhysicalBackupKind {
				return nil
			}
			if owner.APIVersion != mariadbv1alpha1.GroupVersion.String() {
				return nil
			}
			return []string{owner.Name}
		})
}
