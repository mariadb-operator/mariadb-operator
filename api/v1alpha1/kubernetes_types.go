package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// nolint:lll
type VolumeSource struct {
	// mount host directories as read/write.
	// +optional
	HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty" protobuf:"bytes,1,opt,name=hostPath"`
	// emptyDir represents a temporary directory that shares a pod's lifetime.
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty" protobuf:"bytes,2,opt,name=emptyDir"`
	// nfs represents an NFS mount on the host that shares a pod's lifetime
	// +optional
	NFS *corev1.NFSVolumeSource `json:"nfs,omitempty" protobuf:"bytes,7,opt,name=nfs"`
	// persistentVolumeClaimVolumeSource represents a reference to a PersistentVolumeClaim in the same namespace.
	// +optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty" protobuf:"bytes,10,opt,name=persistentVolumeClaim"`
	// csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers (Beta feature).
	// +optional
	CSI *corev1.CSIVolumeSource `json:"csi,omitempty" protobuf:"bytes,28,opt,name=csi"`
}

func (v *VolumeSource) FromKubernetesType(kv *corev1.VolumeSource) {
	v.HostPath = kv.HostPath
	v.EmptyDir = kv.EmptyDir
	v.NFS = kv.NFS
	v.CSI = kv.CSI
}

func (v *VolumeSource) ToKubernetesType() *corev1.VolumeSource {
	return &corev1.VolumeSource{
		HostPath:              v.HostPath,
		EmptyDir:              v.EmptyDir,
		NFS:                   v.NFS,
		PersistentVolumeClaim: v.PersistentVolumeClaim,
		CSI:                   v.CSI,
	}
}
