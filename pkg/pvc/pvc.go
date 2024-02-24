package pvc

import corev1 "k8s.io/api/core/v1"

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
