package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// nolint:lll
// Represents the source of a volume to mount. Only one of its members may be specified.
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

func VolumeSourceFromKubernetesType(kv corev1.VolumeSource) VolumeSource {
	return VolumeSource{
		HostPath:              kv.HostPath,
		EmptyDir:              kv.EmptyDir,
		NFS:                   kv.NFS,
		PersistentVolumeClaim: kv.PersistentVolumeClaim,
		CSI:                   kv.CSI,
	}
}

func (v VolumeSource) ToKubernetesType() corev1.VolumeSource {
	return corev1.VolumeSource{
		HostPath:              v.HostPath,
		EmptyDir:              v.EmptyDir,
		NFS:                   v.NFS,
		PersistentVolumeClaim: v.PersistentVolumeClaim,
		CSI:                   v.CSI,
	}
}

// Volume represents a named volume in a pod that may be accessed by any container in the pod.
type Volume struct {
	// name of the volume.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// volumeSource represents the location and type of the mounted volume.
	VolumeSource `json:",inline" protobuf:"bytes,2,opt,name=volumeSource"`
}

func (v Volume) ToKubernetesType() corev1.Volume {
	return corev1.Volume{
		Name:         v.Name,
		VolumeSource: v.VolumeSource.ToKubernetesType(),
	}
}
