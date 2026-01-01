package v1alpha1

import (
	"errors"
	"fmt"
	"time"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceMonitor defines a prometheus ServiceMonitor object.
type ServiceMonitor struct {
	// PrometheusRelease is the release label to add to the ServiceMonitor object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PrometheusRelease string `json:"prometheusRelease,omitempty"`
	// JobLabel to add to the ServiceMonitor object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	JobLabel string `json:"jobLabel,omitempty"`
	// Interval for scraping metrics.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Interval string `json:"interval,omitempty"`
	// ScrapeTimeout defines the timeout for scraping metrics.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ScrapeTimeout string `json:"scrapeTimeout,omitempty"`
}

// MariadbMetrics defines the metrics for a MariaDB.
type MariadbMetrics struct {
	// Enabled is a flag to enable Metrics
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
	// Exporter defines the metrics exporter container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Exporter Exporter `json:"exporter"`
	// ServiceMonitor defines the ServiceMonior object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServiceMonitor ServiceMonitor `json:"serviceMonitor"`
	// Username is the username of the monitoring user used by the exporter.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Username string `json:"username,omitempty" webhook:"inmutableinit"`
	// PasswordSecretKeyRef is a reference to the password of the monitoring user used by the exporter.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PasswordSecretKeyRef GeneratedSecretKeyRef `json:"passwordSecretKeyRef,omitempty"`
}

// Storage defines the storage options to be used for provisioning the PVCs mounted by MariaDB.
type Storage struct {
	// Ephemeral indicates whether to use ephemeral storage in the PVCs. It is only compatible with non HA MariaDBs.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	Ephemeral *bool `json:"ephemeral,omitempty" webhook:"inmutableinit"`
	// Size of the PVCs to be mounted by MariaDB. Required if not provided in 'VolumeClaimTemplate'. It supersedes the storage size specified in 'VolumeClaimTemplate'.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Size *resource.Quantity `json:"size,omitempty"`
	// StorageClassName to be used to provision the PVCS. It supersedes the 'StorageClassName' specified in 'VolumeClaimTemplate'.
	// If not provided, the default 'StorageClass' configured in the cluster is used.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	StorageClassName string `json:"storageClassName,omitempty" webhook:"inmutable"`
	// ResizeInUseVolumes indicates whether the PVCs can be resized. The 'StorageClassName' used should have 'allowVolumeExpansion' set to 'true' to allow resizing.
	// It defaults to true.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ResizeInUseVolumes *bool `json:"resizeInUseVolumes,omitempty"`
	// WaitForVolumeResize indicates whether to wait for the PVCs to be resized before marking the MariaDB object as ready. This will block other operations such as cluster recovery while the resize is in progress.
	// It defaults to true.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	WaitForVolumeResize *bool `json:"waitForVolumeResize,omitempty"`
	// VolumeClaimTemplate provides a template to define the PVCs.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	VolumeClaimTemplate *VolumeClaimTemplate `json:"volumeClaimTemplate,omitempty"`
	// PersistentVolumeClaimRetentionPolicy describes the lifecycle of PVCs created from volumeClaimTemplates.
	// By default, all persistent volume claims are created as needed and retained until manually deleted.
	// This policy allows the lifecycle to be altered, for example by deleting PVCs when their statefulset is deleted,
	// or when their pod is scaled down.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PVCRetentionPolicy *StatefulSetPersistentVolumeClaimRetentionPolicy `json:"pvcRetentionPolicy,omitempty"`
}

// Storage determines whether a Storage object is valid.
func (s *Storage) Validate(mdb *MariaDB) error {
	if s.Ephemeral != nil {
		if *s.Ephemeral && mdb.IsHAEnabled() {
			return errors.New("ephemeral storage is only compatible with non HA MariaDBs")
		}
		if *s.Ephemeral && (s.Size != nil || s.VolumeClaimTemplate != nil) {
			return errors.New("either ephemeral or regular storage must be provided")
		}
		if *s.Ephemeral {
			return nil
		}
	}
	if s.Size != nil && s.Size.IsZero() {
		return errors.New("greater than zero storage size must be provided")
	}
	if s.Size == nil && s.VolumeClaimTemplate == nil {
		return errors.New("either storage size or volumeClaimTemplate must be provided")
	}
	if s.Size != nil && s.VolumeClaimTemplate != nil {
		vctplSize, ok := s.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage]
		if !ok {
			return nil
		}
		if s.Size.Cmp(vctplSize) < 0 {
			return errors.New("storage size cannot be decreased")
		}
	}
	return nil
}

// SetDefaults sets reasonable defaults.
func (s *Storage) SetDefaults() {
	if s.Ephemeral == nil {
		s.Ephemeral = ptr.To(false)
	}
	if s.ResizeInUseVolumes == nil && !ptr.Deref(s.Ephemeral, false) {
		s.ResizeInUseVolumes = ptr.To(true)
	}
	if s.WaitForVolumeResize == nil && !ptr.Deref(s.Ephemeral, false) {
		s.WaitForVolumeResize = ptr.To(true)
	}

	if s.shouldUpdateVolumeClaimTemplate() {
		vctpl := VolumeClaimTemplate{
			PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *s.Size,
					},
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
			},
		}
		if s.StorageClassName != "" {
			vctpl.StorageClassName = &s.StorageClassName
		}
		s.VolumeClaimTemplate = &vctpl
	}
}

// GetSize obtains the size of the PVC.
func (s *Storage) GetSize() *resource.Quantity {
	vctpl := ptr.Deref(s.VolumeClaimTemplate, VolumeClaimTemplate{})
	if storage, ok := vctpl.Resources.Requests[corev1.ResourceStorage]; ok {
		return &storage
	}
	if s.Size != nil {
		return s.Size
	}
	return nil
}

func (s *Storage) shouldUpdateVolumeClaimTemplate() bool {
	if s.Size == nil {
		return false
	}
	if s.VolumeClaimTemplate == nil {
		return true
	}

	vctplSize, ok := s.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage]
	if !ok {
		return true
	}
	if s.Size.Cmp(vctplSize) != 0 {
		return true
	}
	return s.StorageClassName != "" && s.StorageClassName != ptr.Deref(s.VolumeClaimTemplate.StorageClassName, "")
}

// BootstrapFrom defines a source to bootstrap MariaDB from.
type BootstrapFrom struct {
	// BackupRef is reference to a backup object. If the Kind is not specified, a logical Backup is assumed.
	// This field takes precedence over S3 and Volume sources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	BackupRef *TypedLocalObjectReference `json:"backupRef,omitempty" webhook:"inmutableinit"`
	// VolumeSnapshotRef is a reference to a VolumeSnapshot object.
	// This field takes precedence over S3 and Volume sources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	VolumeSnapshotRef *LocalObjectReference `json:"volumeSnapshotRef,omitempty" webhook:"inmutableinit"`
	// BackupContentType is the backup content type available in the source to bootstrap from.
	// It is inferred based on the BackupRef and VolumeSnapshotRef fields. If inference is not possible, it defaults to Logical.
	// Set this field explicitly when using physical backups from S3 or Volume sources.
	// +optional
	// +kubebuilder:validation:Enum=Logical;Physical
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	BackupContentType BackupContentType `json:"backupContentType,omitempty" webhook:"inmutableinit"`
	// S3 defines the configuration to restore backups from a S3 compatible storage.
	// This field takes precedence over the Volume source.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	S3 *S3 `json:"s3,omitempty" webhook:"inmutableinit"`
	// Volume is a Kubernetes Volume object that contains a backup.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Volume *StorageVolumeSource `json:"volume,omitempty" webhook:"inmutableinit"`
	// TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.
	// It is used to determine the closest restoration source in time.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	TargetRecoveryTime *metav1.Time `json:"targetRecoveryTime,omitempty" webhook:"inmutable"`
	// StagingStorage defines the temporary storage used to keep external backups (i.e. S3) while they are being processed.
	// It defaults to an emptyDir volume, meaning that the backups will be temporarily stored in the node where the Job is scheduled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	StagingStorage *BackupStagingStorage `json:"stagingStorage,omitempty" webhook:"inmutable"`
	// RestoreJob defines additional properties for the Job used to perform the restoration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	RestoreJob *Job `json:"restoreJob,omitempty"`
}

func (b *BootstrapFrom) Validate() error {
	if b.BackupRef == nil && b.VolumeSnapshotRef == nil && b.S3 == nil && b.Volume == nil {
		return errors.New("unable to determine bootstrap source")
	}

	if b.BackupContentType != "" {
		if err := b.BackupContentType.Validate(); err != nil {
			return fmt.Errorf("invalid 'backupContentType': %v", err)
		}
	}

	if b.BackupRef != nil {
		kind := b.BackupRef.Kind

		switch kind {
		case "", BackupKind:
			if b.BackupContentType != "" && b.BackupContentType != BackupContentTypeLogical {
				return fmt.Errorf(
					"inconsistent 'backupRef.kind'='%s' and 'backupContentType'='%s' fields. Logical type must be set in this case",
					kind,
					b.BackupContentType,
				)
			}
		case PhysicalBackupKind:
			if b.BackupContentType != "" && b.BackupContentType != BackupContentTypePhysical {
				return fmt.Errorf(
					"inconsistent 'backupRef.kind'='%s' and 'backupContentType'='%s' fields. Physical type must be set in this case",
					kind,
					b.BackupContentType,
				)
			}
		default:
			return fmt.Errorf("unsupported backup kind: '%v', supported kinds: [%v|%v]", kind, BackupKind, PhysicalBackupKind)
		}
	}

	if b.VolumeSnapshotRef != nil {
		if b.BackupContentType != "" && b.BackupContentType != BackupContentTypePhysical {
			return errors.New("inconsistent 'volumeSnapshotRef' and 'backupContentType' fields. Physical type must be set in this case")
		}
		if b.S3 != nil || b.Volume != nil || b.RestoreJob != nil {
			return errors.New("'s3', 'volume' and 'restoreJob' may not be set when 'volumeSnapshotRef' is set")
		}
	}
	return nil
}

func (b *BootstrapFrom) IsDefaulted() bool {
	return b.Volume != nil || b.VolumeSnapshotRef != nil
}

func (b *BootstrapFrom) SetDefaults(mariadb *MariaDB) {
	if b.BackupRef != nil && b.BackupContentType == "" {
		switch b.BackupRef.Kind {
		case BackupKind:
			b.BackupContentType = BackupContentTypeLogical
		case PhysicalBackupKind:
			b.BackupContentType = BackupContentTypePhysical
		}
	}
	if b.VolumeSnapshotRef != nil && b.BackupContentType == "" {
		b.BackupContentType = BackupContentTypePhysical
	}
	if b.BackupContentType == "" {
		b.BackupContentType = BackupContentTypeLogical
	}
	if b.BackupContentType == BackupContentTypePhysical && b.S3 != nil {
		stagingStorage := ptr.Deref(b.StagingStorage, BackupStagingStorage{})
		b.Volume = ptr.To(stagingStorage.VolumeOrEmptyDir(mariadb.PhysicalBackupStagingPVCKey()))
	}
}

func (b *BootstrapFrom) SetDefaultsWithPhysicalBackup(physicalBackup *PhysicalBackup) error {
	volume, err := physicalBackup.Volume()
	if err != nil {
		return fmt.Errorf("error getting BackupSource volume: %v", err)
	}
	b.Volume = &volume
	b.S3 = physicalBackup.Spec.Storage.S3
	return nil
}

func (b *BootstrapFrom) SetDefaultsWithVolumeSnapshotRef(ref *LocalObjectReference) {
	b.VolumeSnapshotRef = ref
}

func (b *BootstrapFrom) TargetRecoveryTimeOrDefault() time.Time {
	if b.TargetRecoveryTime != nil {
		return b.TargetRecoveryTime.Time
	}
	return time.Now()
}

func (b *BootstrapFrom) RestoreSource() (*RestoreSource, error) {
	var backupRef *LocalObjectReference
	if b.BackupRef != nil {
		if b.BackupRef.Kind == PhysicalBackupKind {
			return nil, errors.New("restoration from physical backups via RestoreSource is not supported")
		}
		backupRef = &LocalObjectReference{
			Name: b.BackupRef.Name,
		}
	}
	return &RestoreSource{
		BackupRef:          backupRef,
		S3:                 b.S3,
		Volume:             b.Volume,
		TargetRecoveryTime: b.TargetRecoveryTime,
		StagingStorage:     b.StagingStorage,
	}, nil
}

// UpdateType defines the type of update for a MariaDB resource.
type UpdateType string

const (
	// ReplicasFirstPrimaryLastUpdateType indicates that the update will be applied to all replica Pods first and later on to the primary Pod.
	// The updates are applied one by one waiting until each Pod passes the readiness probe
	// i.e. the Pod gets synced and it is ready to receive traffic.
	ReplicasFirstPrimaryLastUpdateType UpdateType = "ReplicasFirstPrimaryLast"
	// RollingUpdateUpdateType indicates that the update will be applied by the StatefulSet controller using the RollingUpdate strategy.
	// This strategy is unaware of the roles that the Pod have (primary or replica) and it will
	// perform the update following the StatefulSet ordinal, from higher to lower.
	RollingUpdateUpdateType UpdateType = "RollingUpdate"
	// OnDeleteUpdateType indicates that the update will be applied by the StatefulSet controller using the OnDelete strategy.
	// The update will be done when the Pods get manually deleted by the user.
	OnDeleteUpdateType UpdateType = "OnDelete"
	// NeverUpdateType indicates that the StatefulSet will never be updated.
	// This can be used to roll out updates progressively to a fleet of instances.
	NeverUpdateType UpdateType = "Never"
)

// UpdateStrategy defines how a MariaDB resource is updated.
type UpdateStrategy struct {
	// Type defines the type of updates. One of `ReplicasFirstPrimaryLast`, `RollingUpdate` or `OnDelete`. If not defined, it defaults to `ReplicasFirstPrimaryLast`.
	// +optional
	// +kubebuilder:default=ReplicasFirstPrimaryLast
	// +kubebuilder:validation:Enum=ReplicasFirstPrimaryLast;RollingUpdate;OnDelete;Never
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Type UpdateType `json:"type,omitempty"`
	// RollingUpdate defines parameters for the RollingUpdate type.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	RollingUpdate *appsv1.RollingUpdateStatefulSetStrategy `json:"rollingUpdate,omitempty"`
	// AutoUpdateDataPlane indicates whether the Galera data-plane version (agent and init containers) should be automatically updated based on the operator version. It defaults to false.
	// Updating the operator will trigger updates on all the MariaDB instances that have this flag set to true. Thus, it is recommended to progressively set this flag after having updated the operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AutoUpdateDataPlane *bool `json:"autoUpdateDataPlane,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (u *UpdateStrategy) SetDefaults() {
	if u.Type == "" {
		u.Type = ReplicasFirstPrimaryLastUpdateType
	}
	if u.AutoUpdateDataPlane == nil {
		u.AutoUpdateDataPlane = ptr.To(false)
	}
}

// TLS defines the PKI to be used with MariaDB.
type TLS struct {
	// Enabled indicates whether TLS is enabled, determining if certificates should be issued and mounted to the MariaDB instance.
	// It is enabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`
	// Required specifies whether TLS must be enforced for all connections.
	// User TLS requirements take precedence over this.
	// It disabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Required *bool `json:"required,omitempty"`
	// ServerCASecretRef is a reference to a Secret containing the server certificate authority keypair. It is used to establish trust and issue server certificates.
	// One of:
	// - Secret containing both the 'ca.crt' and 'ca.key' keys. This allows you to bring your own CA to Kubernetes to issue certificates.
	// - Secret containing only the 'ca.crt' in order to establish trust. In this case, either serverCertSecretRef or serverCertIssuerRef must be provided.
	// If not provided, a self-signed CA will be provisioned to issue the server certificate.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerCASecretRef *LocalObjectReference `json:"serverCASecretRef,omitempty"`
	// ServerCertSecretRef is a reference to a TLS Secret containing the server certificate.
	// It is mutually exclusive with serverCertIssuerRef.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerCertSecretRef *LocalObjectReference `json:"serverCertSecretRef,omitempty"`
	// ServerCertIssuerRef is a reference to a cert-manager issuer object used to issue the server certificate. cert-manager must be installed previously in the cluster.
	// It is mutually exclusive with serverCertSecretRef.
	// By default, the Secret field 'ca.crt' provisioned by cert-manager will be added to the trust chain. A custom trust bundle may be specified via serverCASecretRef.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerCertIssuerRef *cmmeta.ObjectReference `json:"serverCertIssuerRef,omitempty"`
	// ClientCASecretRef is a reference to a Secret containing the client certificate authority keypair. It is used to establish trust and issue client certificates.
	// One of:
	// - Secret containing both the 'ca.crt' and 'ca.key' keys. This allows you to bring your own CA to Kubernetes to issue certificates.
	// - Secret containing only the 'ca.crt' in order to establish trust. In this case, either clientCertSecretRef or clientCertIssuerRef fields must be provided.
	// If not provided, a self-signed CA will be provisioned to issue the client certificate.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientCASecretRef *LocalObjectReference `json:"clientCASecretRef,omitempty"`
	// ClientCertSecretRef is a reference to a TLS Secret containing the client certificate.
	// It is mutually exclusive with clientCertIssuerRef.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientCertSecretRef *LocalObjectReference `json:"clientCertSecretRef,omitempty"`
	// ClientCertIssuerRef is a reference to a cert-manager issuer object used to issue the client certificate. cert-manager must be installed previously in the cluster.
	// It is mutually exclusive with clientCertSecretRef.
	// By default, the Secret field 'ca.crt' provisioned by cert-manager will be added to the trust chain. A custom trust bundle may be specified via clientCASecretRef.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientCertIssuerRef *cmmeta.ObjectReference `json:"clientCertIssuerRef,omitempty"`
	// GaleraSSTEnabled determines whether Galera SST connections should use TLS.
	// It disabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	GaleraSSTEnabled *bool `json:"galeraSSTEnabled,omitempty"`
}

// MariaDBSpec defines the desired state of MariaDB
type MariaDBSpec struct {
	// ContainerTemplate defines templates to configure Container objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerTemplate `json:",inline"`
	// PodTemplate defines templates to configure Pod objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodTemplate `json:",inline"`
	// SuspendTemplate defines whether the MariaDB reconciliation loop is enabled. This can be useful for maintenance, as disabling the reconciliation loop prevents the operator from interfering with user operations during maintenance activities.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SuspendTemplate `json:",inline"`
	// Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`.
	// Only MariaDB official images are supported.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Image string `json:"image,omitempty"`
	// ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`.
	// +optional
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:imagePullPolicy","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// InheritMetadata defines the metadata to be inherited by children resources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InheritMetadata *Metadata `json:"inheritMetadata,omitempty"`
	// RootPasswordSecretKeyRef is a reference to a Secret key containing the root password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	RootPasswordSecretKeyRef GeneratedSecretKeyRef `json:"rootPasswordSecretKeyRef,omitempty" webhook:"inmutableinit"`
	// RootEmptyPassword indicates if the root password should be empty. Don't use this feature in production, it is only intended for development and test environments.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch", "urn:alm:descriptor:com.tectonic.ui:advanced"}
	RootEmptyPassword *bool `json:"rootEmptyPassword,omitempty" webhook:"inmutableinit"`
	// Database is the name of the initial Database.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Database *string `json:"database,omitempty" webhook:"inmutable"`
	// Username is the initial username to be created by the operator once MariaDB is ready.
	// The initial User will have ALL PRIVILEGES in the initial Database.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Username *string `json:"username,omitempty" webhook:"inmutable"`
	// PasswordSecretKeyRef is a reference to a Secret that contains the password to be used by the initial User.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PasswordSecretKeyRef *GeneratedSecretKeyRef `json:"passwordSecretKeyRef,omitempty"`
	// PasswordHashSecretKeyRef is a reference to the password hash to be used by the initial User.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password hash.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PasswordHashSecretKeyRef *SecretKeySelector `json:"passwordHashSecretKeyRef,omitempty"`
	// PasswordPlugin is a reference to the password plugin and arguments to be used by the initial User.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PasswordPlugin *PasswordPlugin `json:"passwordPlugin,omitempty"`
	// CleanupPolicy defines the behavior for cleaning up the initial User, Database, and Grant created by the operator.
	// +optional
	// +kubebuilder:validation:Enum=Skip;Delete
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	CleanupPolicy *CleanupPolicy `json:"cleanupPolicy,omitempty"`
	// MyCnf allows to specify the my.cnf file mounted by Mariadb.
	// Updating this field will trigger an update to the Mariadb resource.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MyCnf *string `json:"myCnf,omitempty"`
	// MyCnfConfigMapKeyRef is a reference to the my.cnf config file provided via a ConfigMap.
	// If not provided, it will be defaulted with a reference to a ConfigMap containing the MyCnf field.
	// If the referred ConfigMap is labeled with "k8s.mariadb.com/watch", an update to the Mariadb resource will be triggered when the ConfigMap is updated.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	MyCnfConfigMapKeyRef *ConfigMapKeySelector `json:"myCnfConfigMapKeyRef,omitempty"`
	// TimeZone sets the default timezone. If not provided, it defaults to SYSTEM and the timezone data is not loaded.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	TimeZone *string `json:"timeZone,omitempty" webhook:"inmutable"`
	// BootstrapFrom defines a source to bootstrap from.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	BootstrapFrom *BootstrapFrom `json:"bootstrapFrom,omitempty"`
	// Storage defines the storage options to be used for provisioning the PVCs mounted by MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Storage Storage `json:"storage"`
	// Metrics configures metrics and how to scrape them.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Metrics *MariadbMetrics `json:"metrics,omitempty"`
	// TLS defines the PKI to be used with MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	TLS *TLS `json:"tls,omitempty"`
	// Replication configures high availability via replication. This feature is still in alpha, use Galera if you are looking for a more production-ready HA.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Replication *Replication `json:"replication,omitempty"`
	// Replication configures high availability via Galera.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Galera *Galera `json:"galera,omitempty"`
	// MaxScaleRef is a reference to a MaxScale resource to be used with the current MariaDB.
	// Providing this field implies delegating high availability tasks such as primary failover to MaxScale.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MaxScaleRef *ObjectReference `json:"maxScaleRef,omitempty"`
	// Replicas indicates the number of desired instances.
	// +kubebuilder:default=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// disables the validation check for an odd number of replicas.
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicasAllowEvenNumber bool `json:"replicasAllowEvenNumber,omitempty"`
	// Port where the instances will be listening for connections.
	// +optional
	// +kubebuilder:default=3306
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	Port int32 `json:"port,omitempty"`
	// ServicePorts is the list of additional named ports to be added to the Services created by the operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServicePorts []ServicePort `json:"servicePorts,omitempty"`
	// PodDisruptionBudget defines the budget for replica availability.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodDisruptionBudget *PodDisruptionBudget `json:"podDisruptionBudget,omitempty"`
	// UpdateStrategy defines how a MariaDB resource is updated.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`
	// Service defines a template to configure the general Service object.
	// The network traffic of this Service will be routed to all Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Service *ServiceTemplate `json:"service,omitempty"`
	// Connection defines a template to configure the general Connection object.
	// This Connection provides the initial User access to the initial Database.
	// It will make use of the Service to route network traffic to all Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Connection *ConnectionTemplate `json:"connection,omitempty" webhook:"inmutable"`
	// PrimaryService defines a template to configure the primary Service object.
	// The network traffic of this Service will be routed to the primary Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PrimaryService *ServiceTemplate `json:"primaryService,omitempty"`
	// PrimaryConnection defines a template to configure the primary Connection object.
	// This Connection provides the initial User access to the initial Database.
	// It will make use of the PrimaryService to route network traffic to the primary Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PrimaryConnection *ConnectionTemplate `json:"primaryConnection,omitempty" webhook:"inmutable"`
	// SecondaryService defines a template to configure the secondary Service object.
	// The network traffic of this Service will be routed to the secondary Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SecondaryService *ServiceTemplate `json:"secondaryService,omitempty"`
	// SecondaryConnection defines a template to configure the secondary Connection object.
	// This Connection provides the initial User access to the initial Database.
	// It will make use of the SecondaryService to route network traffic to the secondary Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SecondaryConnection *ConnectionTemplate `json:"secondaryConnection,omitempty" webhook:"inmutable"`
}

// MariaDBTLSStatus aggregates the status of the certificates used by the MariaDB instance.
type MariaDBTLSStatus struct {
	// CABundle is the status of the Certificate Authority bundle.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	CABundle []CertificateStatus `json:"caBundle,omitempty"`
	// ServerCert is the status of the server certificate.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ServerCert *CertificateStatus `json:"serverCert,omitempty"`
	// ClientCert is the status of the client certificate.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ClientCert *CertificateStatus `json:"clientCert,omitempty"`
}

// MariaDBStatus defines the observed state of MariaDB
type MariaDBStatus struct {
	// Conditions for the Mariadb object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Replicas indicates the number of current instances.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// CurrentPrimaryPodIndex is the primary Pod index.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	CurrentPrimaryPodIndex *int `json:"currentPrimaryPodIndex,omitempty"`
	// CurrentPrimary is the primary Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes:Pod"}
	CurrentPrimary *string `json:"currentPrimary,omitempty"`
	// CurrentPrimaryFailingSince is the timestamp of the moment when the primary became not ready.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	CurrentPrimaryFailingSince *metav1.Time `json:"currentPrimaryFailingSince,omitempty"`
	// ScaleOutInitialIndex is the initial index where the scale out operation started.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ScaleOutInitialIndex *int `json:"scaleOutInitialIndex,omitempty"`
	// GaleraRecovery is the Galera recovery current state.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	GaleraRecovery *GaleraRecoveryStatus `json:"galeraRecovery,omitempty"`
	// Replication is the replication current status per each Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Replication *ReplicationStatus `json:"replication,omitempty"`
	// DefaultVersion is the MariaDB version used by the operator when it cannot infer the version
	// from spec.image. This can happen if the image uses a digest (e.g. sha256) instead
	// of a version tag.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	DefaultVersion string `json:"defaultVersion,omitempty"`
	// TLS aggregates the status of the certificates used by the MariaDB instance.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	TLS *MariaDBTLSStatus `json:"tls,omitempty"`
}

// SetCondition sets a status condition to MariaDB
func (s *MariaDBStatus) SetCondition(condition metav1.Condition) {
	if s.Conditions == nil {
		s.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&s.Conditions, condition)
}

// RemoveCondition removes a status condition to MariaDB, returning true when removed
func (s *MariaDBStatus) RemoveCondition(conditionType string) bool {
	return meta.RemoveStatusCondition(&s.Conditions, conditionType)
}

// UpdateCurrentPrimary updates the current primary status.
func (s *MariaDBStatus) UpdateCurrentPrimary(mariadb *MariaDB, index int) {
	s.CurrentPrimaryPodIndex = &index
	currentPrimary := statefulset.PodName(mariadb.ObjectMeta, index)
	s.CurrentPrimary = &currentPrimary
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=mdb
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Primary",type="string",JSONPath=".status.currentPrimary"
// +kubebuilder:printcolumn:name="Updates",type="string",JSONPath=".spec.updateStrategy.type"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{MariaDB,v1alpha1},{MaxScale,v1alpha1},{Connection,v1alpha1},{Restore,v1alpha1},{User,v1alpha1},{Grant,v1alpha1},{ConfigMap,v1},{Service,v1},{Secret,v1},{Event,v1},{ServiceAccount,v1},{StatefulSet,v1},{Deployment,v1},{Job,v1},{PodDisruptionBudget,v1},{Role,v1},{RoleBinding,v1},{ClusterRoleBinding,v1}}

// MariaDB is the Schema for the mariadbs API. It is used to define MariaDB clusters.
type MariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:rule="!has(self.galera) || !self.galera.enabled || (self.replicas % 2 == 1 || self.replicasAllowEvenNumber)", message="An odd number of MariaDB instances (mariadb.spec.replicas) is required to avoid split brain situations for Galera. Use 'mariadb.spec.replicasAllowEvenNumber: true' to disable this validation."
	Spec   MariaDBSpec   `json:"spec"`
	Status MariaDBStatus `json:"status,omitempty"`
}

// nolint:gocyclo
// SetDefaults sets reasonable defaults.
func (m *MariaDB) SetDefaults(env *environment.OperatorEnv) error {
	if m.Spec.Image == "" {
		m.Spec.Image = env.RelatedMariadbImage
	}

	if m.Spec.RootEmptyPassword == nil {
		m.Spec.RootEmptyPassword = ptr.To(false)
	}
	if m.Spec.RootPasswordSecretKeyRef == (GeneratedSecretKeyRef{}) && !m.IsRootPasswordEmpty() {
		m.Spec.RootPasswordSecretKeyRef = m.RootPasswordSecretKeyRef()
	}

	if m.Spec.Port == 0 {
		m.Spec.Port = 3306
	}
	if m.Spec.MyCnf != nil && m.Spec.MyCnfConfigMapKeyRef == nil {
		m.Spec.MyCnfConfigMapKeyRef = ptr.To(m.MyCnfConfigMapKeyRef())
	}
	if m.Spec.Username != nil &&
		m.Spec.PasswordSecretKeyRef == nil && m.Spec.PasswordHashSecretKeyRef == nil && m.Spec.PasswordPlugin == nil {
		secretKeyRef := m.PasswordSecretKeyRef()
		m.Spec.PasswordSecretKeyRef = &secretKeyRef
	}
	if m.AreMetricsEnabled() {
		if m.Spec.Metrics.Exporter.Image == "" {
			m.Spec.Metrics.Exporter.Image = env.RelatedExporterImage
		}
		if m.Spec.Metrics.Exporter.Port == 0 {
			m.Spec.Metrics.Exporter.Port = 9104
		}
		if m.Spec.Metrics.Exporter.Affinity != nil {
			m.Spec.Metrics.Exporter.Affinity.SetDefaults(m.Name)
		}
		if m.Spec.Metrics.Username == "" {
			m.Spec.Metrics.Username = m.MetricsKey().Name
		}
		if m.Spec.Metrics.PasswordSecretKeyRef == (GeneratedSecretKeyRef{}) {
			m.Spec.Metrics.PasswordSecretKeyRef = m.MetricsPasswordSecretKeyRef()
		}
	}
	if m.Spec.TLS == nil {
		m.Spec.TLS = &TLS{
			Enabled: true,
		}
	}

	if m.IsGaleraEnabled() {
		if err := m.Spec.Galera.SetDefaults(m, env); err != nil {
			return fmt.Errorf("error setting Galera defaults: %v", err)
		}
	}
	if m.IsReplicationEnabled() {
		if err := m.Spec.Replication.SetDefaults(m, env); err != nil {
			return fmt.Errorf("error setting replication defaults: %v", err)
		}
	}
	if m.Spec.BootstrapFrom != nil {
		m.Spec.BootstrapFrom.SetDefaults(m)
	}

	if m.Spec.UpdateStrategy == (UpdateStrategy{}) {
		m.Spec.UpdateStrategy.SetDefaults()
	}

	m.Spec.Storage.SetDefaults()
	m.Spec.SetDefaults(m.ObjectMeta)

	return nil
}

// IsGaleraEnabled indicates whether the MariaDB instance has Galera enabled
func (m *MariaDB) IsGaleraEnabled() bool {
	return ptr.Deref(m.Spec.Galera, Galera{}).Enabled
}

// IsReplicationEnabled indicates whether the MariaDB instance has replication enabled
func (m *MariaDB) IsReplicationEnabled() bool {
	return ptr.Deref(m.Spec.Replication, Replication{}).Enabled
}

// IsHAEnabled indicates whether the MariaDB instance has HA enabled
func (m *MariaDB) IsHAEnabled() bool {
	return m.IsReplicationEnabled() || m.IsGaleraEnabled()
}

// IsMaxScaleEnabled indicates that a MaxScale instance is forwarding traffic to this MariaDB instance
func (m *MariaDB) IsMaxScaleEnabled() bool {
	return m.Spec.MaxScaleRef != nil
}

// AreMetricsEnabled indicates whether the MariaDB instance has metrics enabled
func (m *MariaDB) AreMetricsEnabled() bool {
	return ptr.Deref(m.Spec.Metrics, MariadbMetrics{}).Enabled
}

// IsInitialUserEnabled indicates whether the initial User is enabled
func (m *MariaDB) IsInitialUserEnabled() bool {
	return m.Spec.Username != nil && m.Spec.Database != nil &&
		(m.Spec.PasswordSecretKeyRef != nil || m.Spec.PasswordHashSecretKeyRef != nil || m.Spec.PasswordPlugin != nil)
}

// IsRootPasswordEmpty indicates whether the MariaDB instance has an empty root password
func (m *MariaDB) IsRootPasswordEmpty() bool {
	return m.Spec.RootEmptyPassword != nil && *m.Spec.RootEmptyPassword
}

// IsRootPasswordDefined indicates whether the MariaDB instance has a root password defined
func (m *MariaDB) IsRootPasswordDefined() bool {
	return m.Spec.RootPasswordSecretKeyRef != (GeneratedSecretKeyRef{})
}

// IsEphemeralStorageEnabled indicates whether the MariaDB instance has ephemeral storage enabled
func (m *MariaDB) IsEphemeralStorageEnabled() bool {
	return ptr.Deref(m.Spec.Storage.Ephemeral, false)
}

// IsTLSEnabled indicates whether TLS is enabled
func (m *MariaDB) IsTLSEnabled() bool {
	return ptr.Deref(m.Spec.TLS, TLS{}).Enabled
}

// IsTLSRequired indicates whether TLS is enabled and must be enforced for all connections.
func (m *MariaDB) IsTLSRequired() bool {
	if !m.IsTLSEnabled() {
		return false
	}
	tls := ptr.Deref(m.Spec.TLS, TLS{})
	return ptr.Deref(tls.Required, false)
}

// IsTLSMutual specifies whether TLS must be mutual between server and client.
func (m *MariaDB) IsTLSMutual() bool {
	return true
}

// IsTLSForGaleraSSTEnabled indicates whether TLS for Galera SSTs is enabled.
func (m *MariaDB) IsTLSForGaleraSSTEnabled() bool {
	if !m.IsGaleraEnabled() || !m.IsTLSEnabled() {
		return false
	}
	tls := ptr.Deref(m.Spec.TLS, TLS{})
	return ptr.Deref(tls.GaleraSSTEnabled, false)
}

// IsReady indicates whether the MariaDB instance is ready
func (m *MariaDB) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReady)
}

// IsRestoringBackup indicates whether the MariaDB instance is restoring backup
func (m *MariaDB) IsRestoringBackup() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeBackupRestored)
}

// HasRestoredBackup indicates whether the MariaDB instance has restored a Backup
func (m *MariaDB) HasRestoredBackup() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeBackupRestored)
}

// IsResizingStorage indicates whether the MariaDB instance is resizing storage
func (m *MariaDB) IsResizingStorage() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeStorageResized)
}

// IsWaitingForStorageResize indicates whether the MariaDB instance is waiting for storage resize
func (m *MariaDB) IsWaitingForStorageResize() bool {
	condition := meta.FindStatusCondition(m.Status.Conditions, ConditionTypeStorageResized)
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionFalse && condition.Reason == ConditionReasonWaitStorageResize
}

// HasPendingUpdate indicates that MariaDB has a pending update.
func (m *MariaDB) HasPendingUpdate() bool {
	condition := meta.FindStatusCondition(m.Status.Conditions, ConditionTypeUpdated)
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionFalse && condition.Reason == ConditionReasonPendingUpdate
}

// IsUpdating indicates that a MariaDB update is in progress.
func (m *MariaDB) IsUpdating() bool {
	condition := meta.FindStatusCondition(m.Status.Conditions, ConditionTypeUpdated)
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionFalse && condition.Reason == ConditionReasonUpdating
}

// IsSuspended whether a MariaDB is suspended.
func (m *MariaDB) IsSuspended() bool {
	return m.Spec.Suspend
}

// IsInitialized indicates that the MariaDB instance has been successfully initialized.
func (m *MariaDB) IsInitialized() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeInitialized)
}

// IsInitializing indicates that the MariaDB instance is being initialized.
func (m *MariaDB) IsInitializing() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeInitialized)
}

// InitError indicates that the MariaDB instance has an initialization error.
func (m *MariaDB) InitError() error {
	c := meta.FindStatusCondition(m.Status.Conditions, ConditionTypeInitialized)
	if c == nil {
		return nil
	}
	if c.Status == metav1.ConditionFalse && c.Reason == ConditionReasonInitError {
		return errors.New(c.Message)
	}
	return nil
}

// IsScalingOut indicates that the MariaDB instance is being initialized.
func (m *MariaDB) IsScalingOut() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeScaledOut)
}

// ScalingOutError indicates that the MariaDB instance has a scaling out error.
func (m *MariaDB) ScalingOutError() error {
	c := meta.FindStatusCondition(m.Status.Conditions, ConditionTypeScaledOut)
	if c == nil {
		return nil
	}
	if c.Status == metav1.ConditionFalse && c.Reason == ConditionReasonScaleOutError {
		return errors.New(c.Message)
	}
	return nil
}

// ServerDNSNames are the Service DNS names used by server TLS certificates.
func (m *MariaDB) TLSServerDNSNames() []string {
	var names []string
	names = append(names, statefulset.ServiceNameVariants(m.ObjectMeta, m.Name)...)
	names = append(names, statefulset.HeadlessServiceNameVariants(m.ObjectMeta, "*", m.InternalServiceKey().Name)...)
	names = append(names, statefulset.ServiceNameVariants(m.ObjectMeta, m.PrimaryServiceKey().Name)...)
	names = append(names, statefulset.ServiceNameVariants(m.ObjectMeta, m.SecondaryServiceKey().Name)...)
	names = append(names, "localhost")
	return names
}

// TLSClientNames are the names used by client TLS certificates.
func (m *MariaDB) TLSClientNames() []string {
	return []string{fmt.Sprintf("%s-client", m.Name)}
}

// Get image pull policy
func (m *MariaDB) GetImagePullPolicy() corev1.PullPolicy {
	return m.Spec.ImagePullPolicy
}

// Get image pull secrets
func (m *MariaDB) GetImagePullSecrets() []LocalObjectReference {
	return m.Spec.ImagePullSecrets
}

// Get image
func (m *MariaDB) GetImage(env *environment.OperatorEnv) string {
	if m.Spec.Image != "" {
		return m.Spec.Image
	}
	return env.RelatedMariadbImage
}

// Get MariaDB hostname
func (m *MariaDB) GetHost() string {
	if m.IsHAEnabled() {
		return statefulset.ServiceFQDNWithService(
			m.ObjectMeta,
			m.PrimaryServiceKey().Name,
		)
	}
	return statefulset.ServiceFQDN(m.ObjectMeta)
}

// Get MariaDB port
func (m *MariaDB) GetPort() int32 {
	return m.Spec.Port
}

// Get MariaDB replicas
func (m *MariaDB) GetReplicas() int32 {
	return m.Spec.Replicas
}

// Get MariaDB Superuser name
func (m *MariaDB) GetSUName() string {
	return "root"
}

// Get MariaDB Superuser credentials
func (m *MariaDB) GetSUCredential() *SecretKeySelector {
	return &m.Spec.RootPasswordSecretKeyRef.SecretKeySelector
}

// Topology refers to the MariaDB topology
type Topology string

var (
	TopologyGalera      Topology = "galera"
	TopologyReplication Topology = "replication"
)

// Get MariaDB data-plane init container
func (m *MariaDB) GetDataPlaneInitContainer() (*Topology, *InitContainer, error) {
	if !m.IsHAEnabled() {
		return nil, nil, errors.New("high availability must be enabled")
	}
	galera := ptr.Deref(m.Spec.Galera, Galera{})
	if galera.Enabled {
		return &TopologyGalera, &galera.InitContainer, nil
	}
	replication := ptr.Deref(m.Spec.Replication, Replication{})
	if replication.Enabled {
		return &TopologyReplication, &replication.InitContainer, nil
	}
	return nil, nil, errors.New("init container could not be found")
}

// Get MariaDB data-plane agent
func (m *MariaDB) GetDataPlaneAgent() (*Topology, *Agent, error) {
	if !m.IsHAEnabled() {
		return nil, nil, errors.New("high availability must be enabled")
	}
	galera := ptr.Deref(m.Spec.Galera, Galera{})
	if galera.Enabled {
		return &TopologyGalera, &galera.Agent, nil
	}
	replication := ptr.Deref(m.Spec.Replication, Replication{})
	if replication.Enabled {
		return &TopologyReplication, &replication.Agent, nil
	}
	return nil, nil, errors.New("agent could not be found")
}

// +kubebuilder:object:root=true

// MariaDBList contains a list of MariaDB
type MariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MariaDB `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *MariaDBList) ListItems() []client.Object {
	items := make([]client.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&MariaDB{}, &MariaDBList{})
}
