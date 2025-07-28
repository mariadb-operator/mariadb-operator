package pvc

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/labels"
	corev1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// IsResizing returns true if the PVC is resizing
func IsResizing(pvc *corev1.PersistentVolumeClaim) bool {
	return IsPersistentVolumeClaimFileSystemResizePending(pvc) || IsPersistentVolumeClaimResizing(pvc)
}

// IsPersistentVolumeClaimFileSystemResizePending returns true if the PVC has FileSystemResizePending condition set to true
func IsPersistentVolumeClaimFileSystemResizePending(pvc *corev1.PersistentVolumeClaim) bool {
	for _, c := range pvc.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		if c.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
			return true
		}
	}
	return false
}

// IsPersistentVolumeClaimResizing returns true if the PVC has Resizing condition set to true
func IsPersistentVolumeClaimResizing(pvc *corev1.PersistentVolumeClaim) bool {
	for _, condition := range pvc.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		if condition.Type == corev1.PersistentVolumeClaimResizing ||
			condition.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
			return true
		}
	}
	return false
}

// ListStoragePVCs lists the storage PVCs of a given MariaDB instance
func ListStoragePVCs(ctx context.Context, client ctrlclient.Client,
	mariadb *mariadbv1alpha1.MariaDB) ([]corev1.PersistentVolumeClaim, error) {
	list := corev1.PersistentVolumeClaimList{}
	listOpts := &ctrlclient.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mariadb).
				WithPVCRole(builder.StorageVolumeRole).
				Build(),
		),
		Namespace: mariadb.GetNamespace(),
	}
	if err := client.List(ctx, &list, listOpts); err != nil {
		return nil, fmt.Errorf("error listing PVCs: %v", err)
	}
	return list.Items, nil
}
