package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#emptydirvolumesource-v1-core.
type EmptyDirVolumeSource struct {
	// +optional
	Medium corev1.StorageMedium `json:"medium,omitempty"`
	// +optional
	SizeLimit *resource.Quantity `json:"sizeLimit,omitempty"`
}

func (v EmptyDirVolumeSource) ToKubernetesType() corev1.EmptyDirVolumeSource {
	return corev1.EmptyDirVolumeSource{
		Medium:    v.Medium,
		SizeLimit: v.SizeLimit,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nfsvolumesource-v1-core.
type NFSVolumeSource struct {
	Server string `json:"server"`
	Path   string `json:"path"`
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (v NFSVolumeSource) ToKubernetesType() corev1.NFSVolumeSource {
	return corev1.NFSVolumeSource{
		Server:   v.Server,
		Path:     v.Path,
		ReadOnly: v.ReadOnly,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#csivolumesource-v1-core.
type CSIVolumeSource struct {
	Driver string `json:"driver"`
	// +optional
	ReadOnly *bool `json:"readOnly,omitempty"`
	// +optional
	FSType *string `json:"fsType,omitempty"`
	// +optional
	VolumeAttributes map[string]string `json:"volumeAttributes,omitempty"`
	// +optional
	NodePublishSecretRef *LocalObjectReference `json:"nodePublishSecretRef,omitempty"`
}

func (v CSIVolumeSource) ToKubernetesType() corev1.CSIVolumeSource {
	volumeSource := corev1.CSIVolumeSource{
		Driver:           v.Driver,
		ReadOnly:         v.ReadOnly,
		FSType:           v.FSType,
		VolumeAttributes: v.VolumeAttributes,
	}
	if v.NodePublishSecretRef != nil {
		volumeSource.NodePublishSecretRef = ptr.To(v.NodePublishSecretRef.ToKubernetesType())
	}
	return volumeSource
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeclaimvolumesource-v1-core.
type PersistentVolumeClaimVolumeSource struct {
	ClaimName string `json:"claimName"`
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (v PersistentVolumeClaimVolumeSource) ToKubernetesType() corev1.PersistentVolumeClaimVolumeSource {
	return corev1.PersistentVolumeClaimVolumeSource{
		ClaimName: v.ClaimName,
		ReadOnly:  v.ReadOnly,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretvolumesource-v1-core.
type SecretVolumeSource struct {
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

func (v SecretVolumeSource) ToKubernetesType() corev1.SecretVolumeSource {
	return corev1.SecretVolumeSource{
		SecretName: v.SecretName,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#configmapvolumesource-v1-core.
type ConfigMapVolumeSource struct {
	LocalObjectReference `json:",inline"`
}

func (v ConfigMapVolumeSource) ToKubernetesType() corev1.ConfigMapVolumeSource {
	return corev1.ConfigMapVolumeSource{
		LocalObjectReference: v.LocalObjectReference.ToKubernetesType(),
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core.
type VolumeSource struct {
	// +optional
	EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	// +optional
	NFS *NFSVolumeSource `json:"nfs,omitempty"`
	// +optional
	CSI *CSIVolumeSource `json:"csi,omitempty"`
	// +optional
	PersistentVolumeClaim *PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
	// +optional
	Secret *SecretVolumeSource `json:"secret,omitempty"`
	// +optional
	ConfigMap *ConfigMapVolumeSource `json:"configMap,omitempty"`
}

func (v VolumeSource) ToKubernetesType() corev1.VolumeSource {
	var volumeSource corev1.VolumeSource
	if v.EmptyDir != nil {
		volumeSource.EmptyDir = ptr.To(v.EmptyDir.ToKubernetesType())
	}
	if v.NFS != nil {
		volumeSource.NFS = ptr.To(v.NFS.ToKubernetesType())
	}
	if v.CSI != nil {
		volumeSource.CSI = ptr.To(v.CSI.ToKubernetesType())
	}
	if v.PersistentVolumeClaim != nil {
		volumeSource.PersistentVolumeClaim = ptr.To(v.PersistentVolumeClaim.ToKubernetesType())
	}
	if v.Secret != nil {
		volumeSource.Secret = ptr.To(v.Secret.ToKubernetesType())
	}
	if v.ConfigMap != nil {
		volumeSource.ConfigMap = ptr.To(v.ConfigMap.ToKubernetesType())
	}
	return volumeSource
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core.
type Volume struct {
	Name         string `json:"name"`
	VolumeSource `json:",inline"`
}

func (v Volume) ToKubernetesType() corev1.Volume {
	return corev1.Volume{
		Name:         v.Name,
		VolumeSource: v.VolumeSource.ToKubernetesType(),
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core.
type VolumeMount struct {
	// This must match the Name of a Volume.
	Name string `json:"name"`
	// +optional
	ReadOnly  bool   `json:"readOnly,omitempty"`
	MountPath string `json:"mountPath"`
	// +optional
	SubPath string `json:"subPath,omitempty"`
}

func (v VolumeMount) ToKubernetesType() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      v.Name,
		ReadOnly:  v.ReadOnly,
		MountPath: v.MountPath,
		SubPath:   v.SubPath,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeclaimspec-v1-core.
type PersistentVolumeClaimSpec struct {
	// +optional
	// +listType=atomic
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// +optional
	Resources corev1.VolumeResourceRequirements `json:"resources,omitempty"`
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

func (p PersistentVolumeClaimSpec) ToKubernetesType() corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes:      p.AccessModes,
		Selector:         p.Selector,
		Resources:        p.Resources,
		StorageClassName: p.StorageClassName,
	}
}
