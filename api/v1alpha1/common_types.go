package v1alpha1

import (
	"errors"
	"fmt"

	"github.com/mmontes11/mariadb-operator/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	inmutableWebhook = webhook.NewInmutableWebhook(
		webhook.WithTagName("webhook"),
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

type MariaDBRef struct {
	// +kubebuilder:validation:Required
	corev1.LocalObjectReference `json:",inline"`
	// +kubebuilder:default=true
	WaitForIt bool `json:"waitForIt,omitempty"`
}

type SecretTemplate struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Key         *string           `json:"key,omitempty"`
}

type HealthCheck struct {
	Interval      *metav1.Duration `json:"interval,omitempty"`
	RetryInterval *metav1.Duration `json:"retryInterval,omitempty"`
}

type ConnectionTemplate struct {
	SecretName     *string         `json:"secretName,omitempty" webhook:"inmutableinit"`
	SecretTemplate *SecretTemplate `json:"secretTemplate,omitempty" webhook:"inmutableinit"`
	HealthCheck    *HealthCheck    `json:"healthCheck,omitempty"`
}

type RestoreSource struct {
	// It will be used to init the rest of the fields if specified
	BackupRef *corev1.LocalObjectReference `json:"backupRef,omitempty" webhook:"inmutableinit"`
	Volume    *corev1.VolumeSource         `json:"volume,omitempty" webhook:"inmutableinit"`
	// +kubebuilder:default=false
	Physical *bool   `json:"physical,omitempty" webhook:"inmutableinit"`
	FileName *string `json:"fileName,omitempty" webhook:"inmutableinit"`
}

func (r *RestoreSource) IsInit() bool {
	return r.Volume != nil && r.Physical != nil
}

func (r *RestoreSource) Init(backup *Backup) {
	if backup.Spec.Storage.Volume != nil {
		r.Volume = backup.Spec.Storage.Volume
	}
	if backup.Spec.Storage.PersistentVolumeClaim != nil {
		r.Volume = &corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: backup.Name,
			},
		}
	}
	r.Physical = &backup.Spec.Physical
}

func (r *RestoreSource) Validate() error {
	if r.BackupRef != nil {
		return nil
	}
	if r.Volume == nil || r.Physical == nil {
		return errors.New("unable to determine restore source")
	}
	return nil
}
