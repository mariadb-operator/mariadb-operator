// nolint:lll
package v1alpha1

import appsv1 "k8s.io/api/apps/v1"

// PersistentVolumeClaimRetentionPolicyType describes the lifecycle of persistent volume claims.
// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#statefulsetpersistentvolumeclaimretentionpolicy-v1-apps.
type PersistentVolumeClaimRetentionPolicyType string

const (
	// PersistentVolumeClaimRetentionPolicyDelete deletes PVCs when their owning pods or StatefulSet are deleted.
	PersistentVolumeClaimRetentionPolicyDelete PersistentVolumeClaimRetentionPolicyType = "Delete"
	// PersistentVolumeClaimRetentionPolicyRetain retains PVCs when their owning pods or StatefulSet are deleted.
	PersistentVolumeClaimRetentionPolicyRetain PersistentVolumeClaimRetentionPolicyType = "Retain"
)

// StatefulSetPersistentVolumeClaimRetentionPolicy describes the lifecycle of PVCs created from volumeClaimTemplates.
// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#statefulsetpersistentvolumeclaimretentionpolicy-v1-apps.
type StatefulSetPersistentVolumeClaimRetentionPolicy struct {
	// +optional
	WhenDeleted PersistentVolumeClaimRetentionPolicyType `json:"whenDeleted,omitempty"`
	// +optional
	WhenScaled PersistentVolumeClaimRetentionPolicyType `json:"whenScaled,omitempty"`
}

func (p StatefulSetPersistentVolumeClaimRetentionPolicy) ToKubernetesType() appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy {
	return appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
		WhenDeleted: appsv1.PersistentVolumeClaimRetentionPolicyType(p.WhenDeleted),
		WhenScaled:  appsv1.PersistentVolumeClaimRetentionPolicyType(p.WhenScaled),
	}
}
