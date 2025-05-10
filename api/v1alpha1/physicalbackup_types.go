package v1alpha1

import (
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PhysicalBackupSchedule defines when the PhysicalBackup will be taken.
type PhysicalBackupSchedule struct {
	// Schedule contains parameters to define a schedule.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Schedule `json:",inline"`
	// Immediate indicates whether the first backup should be taken immediately after creating the PhysicalBackup.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Immediate *bool `json:"immediate,omitempty"`
}

// PhysicalBackupSpec defines the desired state of PhysicalBackup.
type PhysicalBackupSpec struct {
	// JobContainerTemplate defines templates to configure Container objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	JobContainerTemplate `json:",inline"`
	// JobPodTemplate defines templates to configure Pod objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	JobPodTemplate `json:",inline"`
	// MariaDBRef is a reference to a MariaDB object.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// Compression algorithm to be used in the Backup.
	// +optional
	// +kubebuilder:validation:Enum=none;bzip2;gzip
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Compression CompressAlgorithm `json:"compression,omitempty"`
	// StagingStorage defines the temporary storage used to keep external backups (i.e. S3) while they are being processed.
	// It defaults to an emptyDir volume, meaning that the backups will be temporarily stored in the node where the PhysicalBackup Job is scheduled.
	// The staging area gets cleaned up after each backup is completed, consider this for sizing it appropriately.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	StagingStorage *BackupStagingStorage `json:"stagingStorage,omitempty" webhook:"inmutable"`
	// Storage defines the final storage for backups.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Storage BackupStorage `json:"storage" webhook:"inmutable"`
	// Schedule defines when the PhysicalBackup will be taken.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Schedule *PhysicalBackupSchedule `json:"schedule,omitempty"`
	// MaxRetention defines the retention policy for backups. Old backups will be cleaned up by the Backup Job.
	// It defaults to 30 days.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MaxRetention metav1.Duration `json:"maxRetention,omitempty" webhook:"inmutableinit"`
	// BackoffLimit defines the maximum number of attempts to successfully take a PhysicalBackup.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	BackoffLimit int32 `json:"backoffLimit,omitempty"`
	// RestartPolicy to be added to the PhysicalBackup Pod.
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

// PhysicalBackupStatus defines the observed state of PhysicalBackup.
type PhysicalBackupStatus struct {
	// Conditions for the PhysicalBackup object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// LastScheduleTime is the last time that a Job was scheduled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	// NextScheduleTime is the next time that a Job will be scheduled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	NextScheduleTime *metav1.Time `json:"nextScheduleTime,omitempty"`
}

func (b *PhysicalBackupStatus) SetCondition(condition metav1.Condition) {
	if b.Conditions == nil {
		b.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&b.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=pbmdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Complete",type="string",JSONPath=".status.conditions[?(@.type==\"Complete\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Complete\")].message"
// +kubebuilder:printcolumn:name="MariaDB",type="string",JSONPath=".spec.mariaDbRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{Backup,v1alpha1},{Job,v1},{PersistentVolumeClaim,v1},{ServiceAccount,v1}}

// PhysicalBackup is the Schema for the physicalbackups API. It is used to define physical backup jobs and its storage.
type PhysicalBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PhysicalBackupSpec   `json:"spec,omitempty"`
	Status PhysicalBackupStatus `json:"status,omitempty"`
}

func (b *PhysicalBackup) IsComplete() bool {
	return meta.IsStatusConditionTrue(b.Status.Conditions, ConditionTypeComplete)
}

func (b *PhysicalBackup) Validate() error {
	if b.Spec.Schedule != nil {
		if err := b.Spec.Schedule.Validate(); err != nil {
			return fmt.Errorf("invalid Schedule: %v", err)
		}
	}
	if err := b.Spec.Storage.Validate(); err != nil {
		return fmt.Errorf("invalid Storage: %v", err)
	}
	if err := b.Spec.Compression.Validate(); err != nil {
		return fmt.Errorf("invalid Compression: %v", err)
	}
	if b.Spec.Storage.S3 == nil && b.Spec.StagingStorage != nil {
		return errors.New("'spec.stagingStorage' may only be specified when 'spec.storage.s3' is set")
	}
	return nil
}

func (b *PhysicalBackup) SetDefaults(mariadb *MariaDB) {
	if b.Spec.Compression == CompressAlgorithm("") {
		b.Spec.Compression = CompressNone
	}
	if b.Spec.MaxRetention == (metav1.Duration{}) {
		b.Spec.MaxRetention = metav1.Duration{Duration: 30 * 24 * time.Hour}
	}
	if b.Spec.BackoffLimit == 0 {
		b.Spec.BackoffLimit = 5
	}
	b.Spec.JobPodTemplate.SetDefaults(b.ObjectMeta, mariadb.ObjectMeta)
}

func (b *PhysicalBackup) Volume() (StorageVolumeSource, error) {
	if b.Spec.Storage.S3 != nil {
		stagingStorage := ptr.Deref(b.Spec.StagingStorage, BackupStagingStorage{})
		return stagingStorage.VolumeOrEmptyDir(b.StagingPVCKey()), nil
	}
	if b.Spec.Storage.PersistentVolumeClaim != nil {
		return StorageVolumeSource{
			PersistentVolumeClaim: &PersistentVolumeClaimVolumeSource{
				ClaimName: b.StoragePVCKey().Name,
			},
		}, nil
	}
	if b.Spec.Storage.Volume != nil {
		return *b.Spec.Storage.Volume, nil
	}
	return StorageVolumeSource{}, errors.New("unable to get volume for Backup")
}

// +kubebuilder:object:root=true

// PhysicalBackupList contains a list of PhysicalBackup.
type PhysicalBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PhysicalBackup `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *PhysicalBackupList) ListItems() []client.Object {
	items := make([]client.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&PhysicalBackup{}, &PhysicalBackupList{})
}
