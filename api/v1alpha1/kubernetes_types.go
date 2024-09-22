// nolint:lll
package v1alpha1

import (
	kadapter "github.com/mariadb-operator/mariadb-operator/pkg/kubernetes/adapter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core.
type VolumeSource struct {
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty" protobuf:"bytes,2,opt,name=emptyDir"`
	// +optional
	NFS *corev1.NFSVolumeSource `json:"nfs,omitempty" protobuf:"bytes,7,opt,name=nfs"`
	// +optional
	CSI *corev1.CSIVolumeSource `json:"csi,omitempty" protobuf:"bytes,28,opt,name=csi"`
	// +optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty" protobuf:"bytes,10,opt,name=persistentVolumeClaim"`
}

func VolumeSourceFromKubernetesType(kv corev1.VolumeSource) VolumeSource {
	return VolumeSource{
		EmptyDir:              kv.EmptyDir,
		NFS:                   kv.NFS,
		CSI:                   kv.CSI,
		PersistentVolumeClaim: kv.PersistentVolumeClaim,
	}
}

func (v VolumeSource) ToKubernetesType() corev1.VolumeSource {
	return corev1.VolumeSource{
		EmptyDir:              v.EmptyDir,
		NFS:                   v.NFS,
		CSI:                   v.CSI,
		PersistentVolumeClaim: v.PersistentVolumeClaim,
	}
}

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

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeclaimspec-v1-core
type PersistentVolumeClaimSpec struct {
	// +optional
	// +listType=atomic
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,1,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,4,opt,name=selector"`
	// +optional
	Resources corev1.VolumeResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty" protobuf:"bytes,3,opt,name=storageClassName"`
}

func (p PersistentVolumeClaimSpec) ToKubernetesType() corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes:      p.AccessModes,
		Selector:         p.Selector,
		Resources:        p.Resources,
		StorageClassName: p.StorageClassName,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podaffinityterm-v1-core
type PodAffinityTerm struct {
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty" protobuf:"bytes,1,opt,name=labelSelector"`
	TopologyKey   string                `json:"topologyKey" protobuf:"bytes,2,opt,name=topologyKey"`
}

func (p PodAffinityTerm) ToKubernetesType() corev1.PodAffinityTerm {
	return corev1.PodAffinityTerm{
		LabelSelector: p.LabelSelector,
		TopologyKey:   p.TopologyKey,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#weightedpodaffinityterm-v1-core
type WeightedPodAffinityTerm struct {
	Weight          int32           `json:"weight" protobuf:"varint,1,opt,name=weight"`
	PodAffinityTerm PodAffinityTerm `json:"podAffinityTerm" protobuf:"bytes,2,opt,name=podAffinityTerm"`
}

func (p WeightedPodAffinityTerm) ToKubernetesType() corev1.WeightedPodAffinityTerm {
	return corev1.WeightedPodAffinityTerm{
		Weight:          p.Weight,
		PodAffinityTerm: p.PodAffinityTerm.ToKubernetesType(),
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podantiaffinity-v1-core.
type PodAntiAffinity struct {
	// +optional
	// +listType=atomic
	RequiredDuringSchedulingIgnoredDuringExecution []PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" protobuf:"bytes,1,rep,name=requiredDuringSchedulingIgnoredDuringExecution"`
	// +optional
	// +listType=atomic
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" protobuf:"bytes,2,rep,name=preferredDuringSchedulingIgnoredDuringExecution"`
}

func (p PodAntiAffinity) ToKubernetesType() corev1.PodAntiAffinity {
	return corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution:  kadapter.ToKubernetesSlice(p.RequiredDuringSchedulingIgnoredDuringExecution),
		PreferredDuringSchedulingIgnoredDuringExecution: kadapter.ToKubernetesSlice(p.PreferredDuringSchedulingIgnoredDuringExecution),
	}
}

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
