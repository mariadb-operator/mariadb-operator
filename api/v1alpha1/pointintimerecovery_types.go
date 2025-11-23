package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PointInTimeRecoverySpec defines the desired state of PointInTimeRecovery. It contains binlog archive and point-in-time restoration settings.
type PointInTimeRecoverySpec struct {
	// PhysicalBackupRef is a reference to a PhysicalBackup object that will be used as base backup.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PhysicalBackupRef LocalObjectReference `json:"physicalBackupRef"`
	// S3 is the S3-compatible storage where the binary logs will be kept.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	S3 S3 `json:"s3"`
	// Compression algorithm to be used for compressing the binary logs.
	// +optional
	// +kubebuilder:validation:Enum=none;bzip2;gzip
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Compression CompressAlgorithm `json:"compression,omitempty"`
	// ArchiveTimeout defines the maximum duration for the binary log archival..
	// If this duration is exceeded, the sidecar agent will log an error and it will be retried in the next archive cycle.
	// It defaults to 1 hour.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ArchiveTimeout *metav1.Duration `json:"archiveTimeout,omitempty"`
	// MaxRetention defines the retention policy for binary logs. Old binary logs will be purged after every archive cycle.
	// By default, old binary logs are not purged.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MaxRetention *metav1.Duration `json:"maxRetention,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=pitr
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PointInTimeRecovery is the Schema for the pointintimerecoveries API.  It contains binlog archive and point-in-time restoration settings.
type PointInTimeRecovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PointInTimeRecoverySpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// PointInTimeRecoveryList contains a list of PointInTimeRecovery.
type PointInTimeRecoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PointInTimeRecovery `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PointInTimeRecovery{}, &PointInTimeRecoveryList{})
}
