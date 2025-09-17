package volumesnapshot

import (
	"context"
	"fmt"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/labels"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ListVolumeSnapshots lists the VolumeSnapshots of a given PhysicalBackup instance
func ListVolumeSnapshots(ctx context.Context, client ctrlclient.Client,
	backup *mariadbv1alpha1.PhysicalBackup) (*volumesnapshotv1.VolumeSnapshotList, error) {
	var volumeSnapshotList volumesnapshotv1.VolumeSnapshotList
	if err := client.List(
		ctx,
		&volumeSnapshotList,
		ctrlclient.InNamespace(backup.Namespace),
		ctrlclient.MatchingLabels(
			labels.NewLabelsBuilder().
				WithPhysicalBackupSelectorLabels(backup).
				Build(),
		),
	); err != nil {
		return nil, err
	}
	return &volumeSnapshotList, nil
}

// ListVolumeSnapshots lists the VolumeSnapshots of a given PhysicalBackup instance in a ready state.
func ListReadyVolumeSnapshots(ctx context.Context, client ctrlclient.Client,
	backup *mariadbv1alpha1.PhysicalBackup) (*volumesnapshotv1.VolumeSnapshotList, error) {
	volumeSnapshotList, err := ListVolumeSnapshots(ctx, client, backup)
	if err != nil {
		return nil, fmt.Errorf("error listing VolumeSnapshots: %v", err)
	}
	var items []volumesnapshotv1.VolumeSnapshot
	for _, snapshot := range volumeSnapshotList.Items {
		if IsVolumeSnapshotReady(&snapshot) {
			items = append(items, snapshot)
		}
	}
	return &volumesnapshotv1.VolumeSnapshotList{
		Items: items,
	}, nil
}

// IsVolumeSnapshotProvisioned determines whether a VolumeSnapshot has been provisioned by the storage system
func IsVolumeSnapshotProvisioned(snapshot *volumesnapshotv1.VolumeSnapshot) bool {
	status := ptr.Deref(snapshot.Status, volumesnapshotv1.VolumeSnapshotStatus{})
	boundVolumeSnapshotContentName := ptr.Deref(status.BoundVolumeSnapshotContentName, "")

	if boundVolumeSnapshotContentName == "" {
		return false
	}
	return status.CreationTime != nil && status.Error == nil
}

// IsVolumeSnapshotReady determines whether a VolumeSnapshot is ready.
func IsVolumeSnapshotReady(snapshot *volumesnapshotv1.VolumeSnapshot) bool {
	if !IsVolumeSnapshotProvisioned(snapshot) {
		return false
	}
	status := ptr.Deref(snapshot.Status, volumesnapshotv1.VolumeSnapshotStatus{})
	ready := ptr.Deref(status.ReadyToUse, false)

	return ready && status.Error == nil
}
