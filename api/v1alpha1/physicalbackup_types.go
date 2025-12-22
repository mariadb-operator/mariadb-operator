package v1alpha1

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	mdbtime "github.com/mariadb-operator/mariadb-operator/v25/pkg/time"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// DefaultPhysicalBackupMaxRetention defines the default maximum PhysicalBackup retetion policy.
	DefaultPhysicalBackupMaxRetention = metav1.Duration{Duration: 30 * 24 * time.Hour}
	// DefaultPhysicalBackupTimeout defines the default maximum duration of a PhysicalBackup job or snapshot.
	DefaultPhysicalBackupTimeout = metav1.Duration{Duration: 1 * time.Hour}
)

// PhysicalBackupPodTemplate defines a template to configure Container objects that run in a PhysicalBackup.
type PhysicalBackupPodTemplate struct {
	// PodMetadata defines extra metadata for the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodMetadata *Metadata `json:"podMetadata,omitempty"`
	// ImagePullSecrets is the list of pull Secrets to be used to pull the image.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ImagePullSecrets []LocalObjectReference `json:"imagePullSecrets,omitempty" webhook:"inmutable"`
	// SecurityContext holds pod-level security attributes and common container settings.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodSecurityContext *PodSecurityContext `json:"podSecurityContext,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to be used by the Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServiceAccountName *string `json:"serviceAccountName,omitempty" webhook:"inmutableinit"`
	// Tolerations to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// PriorityClassName to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PriorityClassName *string `json:"priorityClassName,omitempty" webhook:"inmutable"`
}

// SetDefaults sets reasonable defaults.
func (p *PhysicalBackupPodTemplate) SetDefaults(objMeta, mariadbObjMeta metav1.ObjectMeta) {
	if p.ServiceAccountName == nil {
		p.ServiceAccountName = ptr.To(p.ServiceAccountKey(objMeta).Name)
	}
}

// ServiceAccountKey defines the key for the ServiceAccount object.
func (p *PhysicalBackupPodTemplate) ServiceAccountKey(objMeta metav1.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{
		Name:      ptr.Deref(p.ServiceAccountName, objMeta.Name),
		Namespace: objMeta.Namespace,
	}
}

// PhysicalBackupTarget defines in which Pod the physical backups will be taken.
type PhysicalBackupTarget string

const (
	// PhysicalBackupTargetReplica indicates that the physical backup will be taken in a ready replica.
	PhysicalBackupTargetReplica PhysicalBackupTarget = "Replica"
	// PhysicalBackupTargetReplica indicates that the physical backup will preferably be taken in a ready replica.
	// If no ready replicas are available, physical backups will be taken in the primary.
	PhysicalBackupTargetPreferReplica PhysicalBackupTarget = "PreferReplica"
)

func (c PhysicalBackupTarget) Validate() error {
	switch c {
	case PhysicalBackupTargetReplica, PhysicalBackupTargetPreferReplica:
		return nil
	default:
		return fmt.Errorf(
			"invalid physical backup target: %v, supported: [%v|%v]",
			c,
			PhysicalBackupTargetReplica,
			PhysicalBackupTargetPreferReplica,
		)
	}
}

// PhysicalBackupSchedule defines when the PhysicalBackup will be taken.
type PhysicalBackupSchedule struct {
	// Cron is a cron expression that defines the schedule.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Cron string `json:"cron" webhook:"inmutable"`
	// Suspend defines whether the schedule is active or not.
	// +optional
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	Suspend bool `json:"suspend"`
	// Immediate indicates whether the first backup should be taken immediately after creating the PhysicalBackup.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Immediate *bool `json:"immediate,omitempty"`
}

// Validate determines whether a PhysicalBackupSchedule is valid.
func (s *PhysicalBackupSchedule) Validate() error {
	if s.Cron != "" {
		_, err := CronParser.Parse(s.Cron)
		return err
	}
	return nil
}

// PhysicalBackupVolumeSnapshot defines parameters for the VolumeSnapshots used as physical backups.
type PhysicalBackupVolumeSnapshot struct {
	// Metadata is extra metadata to the added to the VolumeSnapshot objects.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Metadata *Metadata `json:"metadata,omitempty"`
	// VolumeSnapshotClassName is the VolumeSnapshot class to be used to take snapshots.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName"`
}

// PhysicalBackupStorage defines the storage for physical backups.
type PhysicalBackupStorage struct {
	// S3 defines the configuration to store backups in a S3 compatible storage.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	S3 *S3 `json:"s3,omitempty"`
	// PersistentVolumeClaim is a Kubernetes PVC specification.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PersistentVolumeClaim *PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
	// Volume is a Kubernetes volume specification.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Volume *StorageVolumeSource `json:"volume,omitempty"`
	// VolumeSnapshot is a Kubernetes VolumeSnapshot specification.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	VolumeSnapshot *PhysicalBackupVolumeSnapshot `json:"volumeSnapshot,omitempty"`
}

func (b *PhysicalBackupStorage) Validate() error {
	storageTypes := 0
	fields := reflect.ValueOf(b).Elem()
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		if !field.IsNil() {
			storageTypes++
		}
	}
	if storageTypes != 1 {
		return errors.New("exactly one storage type should be provided")
	}
	return nil
}

// PhysicalBackupSpec defines the desired state of PhysicalBackup.
type PhysicalBackupSpec struct {
	// JobContainerTemplate defines templates to configure Container objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	JobContainerTemplate `json:",inline"`
	// PhysicalBackupPodTemplate defines templates to configure Pod objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PhysicalBackupPodTemplate `json:",inline"`
	// MariaDBRef is a reference to a MariaDB object.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// Target defines in which Pod the physical backups will be taken. It defaults to "Replica", meaning that the physical backups will only be taken in ready replicas.
	// +optional
	// +kubebuilder:validation:Enum=Replica;PreferReplica
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Target *PhysicalBackupTarget `json:"target,omitempty"`
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
	Storage PhysicalBackupStorage `json:"storage" webhook:"inmutable"`
	// Schedule defines when the PhysicalBackup will be taken.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Schedule *PhysicalBackupSchedule `json:"schedule,omitempty"`
	// MaxRetention defines the retention policy for backups. Old backups will be cleaned up by the Backup Job.
	// It defaults to 30 days.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MaxRetention metav1.Duration `json:"maxRetention,omitempty"`
	// Timeout defines the maximum duration of a PhysicalBackup job or snapshot.
	// If this duration is exceeded, the job or snapshot is considered expired and is deleted by the operator.
	// A new job or snapshot will then be created according to the schedule.
	// It defaults to 1 hour.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// PodAffinity indicates whether the Jobs should run in the same Node as the MariaDB Pods to be able to attach the PVC.
	// It defaults to true.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	PodAffinity *bool `json:"podAffinity,omitempty"`
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
	// SuccessfulJobsHistoryLimit defines the maximum number of successful Jobs to be displayed. It defaults to 5.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`
	// LogLevel to be used in the PhysicalBackup Job. It defaults to 'info'.
	// +optional
	// +kubebuilder:default=info
	// +kubebuilder:validation:Enum=debug;info;warn;error;dpanic;panic;fatal
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	LogLevel string `json:"logLevel,omitempty"`
}

// PhysicalBackupStatus defines the observed state of PhysicalBackup.
type PhysicalBackupStatus struct {
	// Conditions for the PhysicalBackup object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// LastScheduleCheckTime is the last time that the schedule was checked.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastScheduleCheckTime *metav1.Time `json:"lastScheduleCheckTime,omitempty"`
	// LastScheduleTime is the last time that a backup was scheduled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	// NextScheduleTime is the next time that a backup will be scheduled.
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
// +kubebuilder:printcolumn:name="Last Scheduled",type="date",JSONPath=".status.lastScheduleTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{PhysicalBackup,v1alpha1},{Job,v1},{ServiceAccount,v1},{PersistentVolumeClaim,v1}}

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
	if b.Spec.Target != nil {
		if err := b.Spec.Target.Validate(); err != nil {
			return fmt.Errorf("invalid Target: %v", err)
		}
	}
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

	storage := b.Spec.Storage
	if storage.VolumeSnapshot != nil && (storage.S3 != nil || storage.Volume != nil) {
		return errors.New("'s3' and 'volume' storage types may not be set when 'volumeSnapshotRef' is set")
	}
	if storage.S3 == nil && b.Spec.StagingStorage != nil {
		return errors.New("'spec.stagingStorage' may only be specified when 'spec.storage.s3' is set")
	}
	return nil
}

func (b *PhysicalBackup) SetDefaults(mariadb *MariaDB) {
	if b.Spec.Target == nil {
		b.Spec.Target = ptr.To(PhysicalBackupTargetReplica)
	}
	if b.Spec.MaxRetention == (metav1.Duration{}) {
		b.Spec.MaxRetention = DefaultPhysicalBackupMaxRetention
	}
	if b.Spec.Timeout == nil {
		b.Spec.Timeout = &DefaultPhysicalBackupTimeout
	}
	if b.Spec.Storage.VolumeSnapshot != nil {
		return // VolumeSnapshot does not use the rest of the fields, defaulting can be skipped
	}
	if b.Spec.Compression == CompressAlgorithm("") {
		b.Spec.Compression = CompressNone
	}
	if b.Spec.BackoffLimit == 0 {
		b.Spec.BackoffLimit = 5
	}
	if b.Spec.SuccessfulJobsHistoryLimit == nil {
		b.Spec.SuccessfulJobsHistoryLimit = ptr.To(int32(5))
	}
	b.Spec.SetDefaults(b.ObjectMeta, mariadb.ObjectMeta)
}

func (b *PhysicalBackup) Volume() (StorageVolumeSource, error) {
	if b.Spec.Storage.VolumeSnapshot != nil {
		return StorageVolumeSource{}, errors.New("VolumeSnapshot does not require a volume")
	}
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
	return StorageVolumeSource{}, errors.New("unable to get volume for PhysicalBackup")
}

// IsValidPhysicalBackup determines whether a PhysicalBackup name is valid
func IsValidPhysicalBackup(name string) bool {
	_, err := ParsePhysicalBackupTime(name)
	return err == nil
}

// ParsePhysicalBackupTime parses the time from a PhysicalBackup name
func ParsePhysicalBackupTime(name string) (time.Time, error) {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid object name \"%s\"", name)
	}
	return mdbtime.Parse(parts[len(parts)-1])
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
