package v1alpha1

import (
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// RestoreSource defines a source for restoring a logical backup.
type RestoreSource struct {
	// BackupRef is a reference to a Backup object. It has priority over S3 and Volume.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	BackupRef *LocalObjectReference `json:"backupRef,omitempty" webhook:"inmutableinit"`
	// S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	S3 *S3 `json:"s3,omitempty" webhook:"inmutableinit"`
	// Volume is a Kubernetes Volume object that contains a backup.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Volume *StorageVolumeSource `json:"volume,omitempty"`
	// TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.
	// It is used to determine the closest restoration source in time.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	TargetRecoveryTime *metav1.Time `json:"targetRecoveryTime,omitempty" webhook:"inmutable"`
	// StagingStorage defines the temporary storage used to keep external backups (i.e. S3) while they are being processed.
	// It defaults to an emptyDir volume, meaning that the backups will be temporarily stored in the node where the Restore Job is scheduled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	StagingStorage *BackupStagingStorage `json:"stagingStorage,omitempty" webhook:"inmutable"`
}

func (r *RestoreSource) Validate() error {
	if r.BackupRef == nil && r.S3 == nil && r.Volume == nil {
		return errors.New("unable to determine restore source")
	}
	if r.S3 == nil && r.StagingStorage != nil {
		return errors.New("'spec.stagingStorage' may only be specified when 'spec.s3' is set")
	}
	return nil
}

func (r *RestoreSource) IsDefaulted() bool {
	return r.Volume != nil
}

func (r *RestoreSource) SetDefaults(restore *Restore) {
	if r.S3 != nil {
		stagingStorage := ptr.Deref(r.StagingStorage, BackupStagingStorage{})
		r.Volume = ptr.To(stagingStorage.VolumeOrEmptyDir(restore.StagingPVCKey()))
	}
}

func (r *RestoreSource) SetDefaultsWithBackup(backup *Backup) error {
	volume, err := backup.Volume()
	if err != nil {
		return fmt.Errorf("error getting Backup volume: %v", err)
	}
	r.Volume = &volume
	r.S3 = backup.Spec.Storage.S3
	return nil
}

func (r *RestoreSource) TargetRecoveryTimeOrDefault() time.Time {
	if r.TargetRecoveryTime != nil {
		return r.TargetRecoveryTime.Time
	}
	return time.Now()
}

// RestoreSpec defines the desired state of restore
type RestoreSpec struct {
	// JobContainerTemplate defines templates to configure Container objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	JobContainerTemplate `json:",inline"`
	// JobPodTemplate defines templates to configure Pod objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	JobPodTemplate `json:",inline"`
	// RestoreSource defines a source for restoring a MariaDB.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	RestoreSource `json:",inline"`
	// MariaDBRef is a reference to a MariaDB object.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// Database defines the logical database to be restored. If not provided, all databases available in the backup are restored.
	// IMPORTANT: The database must previously exist.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Database string `json:"database,omitempty"`
	// LogLevel to be used n the Backup Job. It defaults to 'info'.
	// +optional
	// +kubebuilder:default=info
	// +kubebuilder:validation:Enum=debug;info;warn;error;dpanic;panic;fatal
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	LogLevel string `json:"logLevel,omitempty"`
	// BackoffLimit defines the maximum number of attempts to successfully perform a Backup.
	// +optional
	// +kubebuilder:default=5
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	BackoffLimit int32 `json:"backoffLimit,omitempty"`
	// RestartPolicy to be added to the Backup Job.
	// +optional
	// +kubebuilder:default=OnFailure
	// +kubebuilder:validation:Enum=Always;OnFailure;Never
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	RestartPolicy corev1.RestartPolicy `json:"restartPolicy,omitempty" webhook:"inmutable"`
	// InheritMetadata defines the metadata to be inherited by children resources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InheritMetadata *Metadata `json:"inheritMetadata,omitempty"`
}

// RestoreStatus defines the observed state of restore
type RestoreStatus struct {
	// Conditions for the Restore object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (r *RestoreStatus) SetCondition(condition metav1.Condition) {
	if r.Conditions == nil {
		r.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&r.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=rmdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Complete",type="string",JSONPath=".status.conditions[?(@.type==\"Complete\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Complete\")].message"
// +kubebuilder:printcolumn:name="MariaDB",type="string",JSONPath=".spec.mariaDbRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{Restore,v1alpha1},{Job,v1},{ServiceAccount,v1}}

// Restore is the Schema for the restores API. It is used to define restore jobs and its restoration source.
type Restore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreSpec   `json:"spec,omitempty"`
	Status RestoreStatus `json:"status,omitempty"`
}

func (r *Restore) IsComplete() bool {
	return meta.IsStatusConditionTrue(r.Status.Conditions, ConditionTypeComplete)
}

func (r *Restore) SetDefaults(mariadb *MariaDB) {
	if r.Spec.BackoffLimit == 0 {
		r.Spec.BackoffLimit = 5
	}
	r.Spec.JobPodTemplate.SetDefaults(r.ObjectMeta, mariadb.ObjectMeta)
}

// +kubebuilder:object:root=true

// RestoreList contains a list of restore
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Restore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Restore{}, &RestoreList{})
}
