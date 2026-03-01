package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PointInTimeRecoverySpec defines the desired state of PointInTimeRecovery. It contains binlog archive and point-in-time restoration settings.
type PointInTimeRecoverySpec struct {
	// PhysicalBackupRef is a reference to a PhysicalBackup object that will be used as base backup.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PhysicalBackupRef LocalObjectReference `json:"physicalBackupRef"`
	// PointInTimeRecoveryStorage is the storage where the point in time recovery data will be stored
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PointInTimeRecoveryStorage PointInTimeRecoveryStorage `json:"storage"`
	// Compression algorithm to be used for compressing the binary logs.
	// This field is immutable, it cannot be updated after creation.
	// +optional
	// +kubebuilder:validation:Enum=none;bzip2;gzip
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Compression CompressAlgorithm `json:"compression,omitempty" webhook:"inmutable"`
	// ArchiveTimeout defines the maximum duration for the binary log archival.
	// If this duration is exceeded, the sidecar agent will log an error and it will be retried in the next archive cycle.
	// It defaults to 1 hour.
	// +kubebuilder:default="1h"
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ArchiveTimeout *metav1.Duration `json:"archiveTimeout,omitempty"`
	// StrictMode controls the behavior when a point-in-time restoration cannot reach the exact target time:
	// When enabled: Returns an error and avoids replaying binary logs if target time is not reached.
	// When disabled (default): Replays available binary logs until the last recoverable time. It logs logs an error if target time is not reached.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	StrictMode bool `json:"strictMode"`
}

// PointInTimeRecoveryStorage stores the different storage options for PITR
type PointInTimeRecoveryStorage struct {
	// S3 is the S3-compatible storage where the binary logs will be kept.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	S3 *S3 `json:"s3,omitempty"`
	// AzureBlob is the Azure Blob Storage where the binary logs will be kept.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AzureBlob *AzureBlob `json:"azureBlob,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=pitr
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Physical Backup",type="string",JSONPath=".spec.physicalBackupRef.name"
// +kubebuilder:printcolumn:name="Last Recoverable Time",type="string",JSONPath=".status.lastRecoverableTime"
// +kubebuilder:printcolumn:name="Strict Mode",type="boolean",JSONPath=".spec.strictMode"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PointInTimeRecovery is the Schema for the pointintimerecoveries API. It contains binlog archival and point-in-time restoration settings.
type PointInTimeRecovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PointInTimeRecoverySpec   `json:"spec,omitempty"`
	Status PointInTimeRecoveryStatus `json:"status,omitempty"`
}

// PointInTimeRecoveryStatus represents the current status of the point-in-time-recovery.
type PointInTimeRecoveryStatus struct {
	// LastRecoverableTime is the most recent recoverable time based on the current state of physical backups and archived binary logs.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastRecoverableTime *string `json:"lastRecoverableTime,omitempty"`
}

// +kubebuilder:object:root=true

// PointInTimeRecoveryList contains a list of PointInTimeRecovery.
type PointInTimeRecoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PointInTimeRecovery `json:"items"`
}

func (b *PointInTimeRecovery) Validate() error {
	if err := b.Spec.PointInTimeRecoveryStorage.Validate(); err != nil {
		return fmt.Errorf("invalid storage: %w", err)
	}

	return nil
}

func (s *PointInTimeRecoveryStorage) Validate() error {
	hasAbs := s.AzureBlob != nil
	hasS3 := s.S3 != nil

	if hasAbs == hasS3 {
		return fmt.Errorf("either s3 or abs must be enabled for Point In Time Recovery")
	}

	return nil
}

func init() {
	SchemeBuilder.Register(&PointInTimeRecovery{}, &PointInTimeRecoveryList{})
}
