package v1alpha1

import (
	"fmt"

	"github.com/mmontes11/mariadb-operator/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	inmutableWebhook = webhook.NewInmutableWebhook(
		webhook.WithTagName("webhook"),
		webhook.WithTagValue("inmutable"),
	)
)

type Image struct {
	// +kubebuilder:validation:Required
	Repository string `json:"repository"`
	// +kubebuilder:default=latest
	Tag string `json:"tag,omitempty"`
	// +kubebuilder:default=IfNotPresent
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

func (i *Image) String() string {
	return fmt.Sprintf("%s:%s", i.Repository, i.Tag)
}

type Storage struct {
	// +kubebuilder:validation:Required
	ClassName string `json:"className"`
	// +kubebuilder:validation:Required
	Size resource.Quantity `json:"size"`
	// +kubebuilder:default={ReadWriteOnce}
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}
