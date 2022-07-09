package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Image struct {
	// +kubebuilder:validation:Required
	Repository string `json:"repository"`
	// +kubebuilder:default=latest
	Tag string `json:"tag,omitempty"`
	// +kubebuilder:default=IfNotPresent
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

type Storage struct {
	// +kubebuilder:validation:Required
	ClassName string `json:"className"`
	// +kubebuilder:validation:Required
	Size resource.Quantity `json:"size"`
	// +kubebuilder:default={ReadWriteOnce}
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}
