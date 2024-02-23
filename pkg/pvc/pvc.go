package pvc

import corev1 "k8s.io/api/core/v1"

func IsResizing(pvc corev1.PersistentVolumeClaim) bool {
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
