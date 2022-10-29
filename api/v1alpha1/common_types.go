package v1alpha1

import (
	"errors"
	"fmt"

	"github.com/mmontes11/mariadb-operator/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
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
	Volume                *corev1.VolumeSource              `json:"volume,omitempty"`
	PersistentVolumeClaim *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
}

func (s *Storage) Validate() error {
	if s.Volume == nil && s.PersistentVolumeClaim == nil {
		return errors.New("no storage type provided")
	}
	return nil
}

type MariaDBRef struct {
	// +kubebuilder:validation:Required
	corev1.LocalObjectReference `json:",inline"`

	// +kubebuilder:default=true
	WaitForIt bool `json:"waitForIt,omitempty"`
}

type BackupMariaDBRef struct {
	// +kubebuilder:validation:Required
	corev1.LocalObjectReference `json:",inline"`
}
