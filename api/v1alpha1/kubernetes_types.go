// nolint:lll
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// Represents the source of a volume to mount. Only one of its members may be specified.
// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core.
type VolumeSource struct {
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty" protobuf:"bytes,2,opt,name=emptyDir"`
	// +optional
	NFS *corev1.NFSVolumeSource `json:"nfs,omitempty" protobuf:"bytes,7,opt,name=nfs"`
	// +optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty" protobuf:"bytes,10,opt,name=persistentVolumeClaim"`
	// +optional
	CSI *corev1.CSIVolumeSource `json:"csi,omitempty" protobuf:"bytes,28,opt,name=csi"`
}

func VolumeSourceFromKubernetesType(kv corev1.VolumeSource) VolumeSource {
	return VolumeSource{
		EmptyDir:              kv.EmptyDir,
		NFS:                   kv.NFS,
		PersistentVolumeClaim: kv.PersistentVolumeClaim,
		CSI:                   kv.CSI,
	}
}

func (v VolumeSource) ToKubernetesType() corev1.VolumeSource {
	return corev1.VolumeSource{
		EmptyDir:              v.EmptyDir,
		NFS:                   v.NFS,
		PersistentVolumeClaim: v.PersistentVolumeClaim,
		CSI:                   v.CSI,
	}
}

// Volume represents a named volume in a pod that may be accessed by any container in the pod.
// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core.
type Volume struct {
	Name         string `json:"name" protobuf:"bytes,1,opt,name=name"`
	VolumeSource `json:",inline" protobuf:"bytes,2,opt,name=volumeSource"`
}

func (v Volume) ToKubernetesType() corev1.Volume {
	return corev1.Volume{
		Name:         v.Name,
		VolumeSource: v.VolumeSource.ToKubernetesType(),
	}
}

// Pod affinity is a group of inter pod affinity scheduling rules.
// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podaffinity-v1-core.
type PodAffinity struct {
	// +optional
	// +listType=atomic
	RequiredDuringSchedulingIgnoredDuringExecution []corev1.PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" protobuf:"bytes,1,rep,name=requiredDuringSchedulingIgnoredDuringExecution"`
	// +optional
	// +listType=atomic
	PreferredDuringSchedulingIgnoredDuringExecution []corev1.WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" protobuf:"bytes,2,rep,name=preferredDuringSchedulingIgnoredDuringExecution"`
}

func (p PodAffinity) ToKubernetesType() corev1.PodAffinity {
	return corev1.PodAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution:  p.RequiredDuringSchedulingIgnoredDuringExecution,
		PreferredDuringSchedulingIgnoredDuringExecution: p.PreferredDuringSchedulingIgnoredDuringExecution,
	}
}

// Pod anti affinity is a group of inter pod anti affinity scheduling rules.
// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podantiaffinity-v1-core.
type PodAntiAffinity struct {
	// +optional
	// +listType=atomic
	RequiredDuringSchedulingIgnoredDuringExecution []corev1.PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" protobuf:"bytes,1,rep,name=requiredDuringSchedulingIgnoredDuringExecution"`
	// +optional
	// +listType=atomic
	PreferredDuringSchedulingIgnoredDuringExecution []corev1.WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" protobuf:"bytes,2,rep,name=preferredDuringSchedulingIgnoredDuringExecution"`
}

func (p PodAntiAffinity) ToKubernetesType() corev1.PodAntiAffinity {
	return corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution:  p.RequiredDuringSchedulingIgnoredDuringExecution,
		PreferredDuringSchedulingIgnoredDuringExecution: p.PreferredDuringSchedulingIgnoredDuringExecution,
	}
}

// Affinity is a group of affinity scheduling rules.
// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core.
type Affinity struct {
	// +optional
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty" protobuf:"bytes,1,opt,name=podAntiAffinity"`
}

func (a Affinity) ToKubernetesType() corev1.Affinity {
	var affinity corev1.Affinity
	if a.PodAntiAffinity != nil {
		affinity.PodAntiAffinity = ptr.To(a.PodAntiAffinity.ToKubernetesType())
	}
	return affinity
}
