package v1alpha1

import (
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ExternalMariaDBSpec defines the desired state of an External MariaDB
type ExternalMariaDBSpec struct {
	// Image name to be used to perform operations on the external MariaDB, for example, for taking backups.
	// The supported format is `<image>:<tag>`. Only MariaDB official images are supported.
	// It has priority over the Version field.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Image string `json:"image,omitempty"`
	// ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`.
	// +optional
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:imagePullPolicy","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// ImagePullSecrets is the list of pull Secrets to be used to pull the image.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ImagePullSecrets []LocalObjectReference `json:"imagePullSecrets,omitempty" webhook:"inmutable"`
	// Version is the MariaDB image version to be used to operate with MariaDB, for example, for taking backups.
	// The MariaDB Community images will be used when providing this field.
	// If not provided, the version will be inferred from the external MariaDB.
	// The Image field has priority over this field.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Version *string `json:"version,omitempty"`
	// InheritMetadata defines the metadata to be inherited by children resources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InheritMetadata *Metadata `json:"inheritMetadata,omitempty"`
	// Hostname of the external MariaDB service.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Host string `json:"host"`
	// Port of the external MariaDB.
	// +optional
	// +kubebuilder:default=3306
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	Port int32 `json:"port,omitempty"`
	// Username is the username to connect to the external MariaDB.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Username *string `json:"username"`
	// PasswordSecretKeyRef is a reference to the password to be used by the User.
	// If not provided, the account will be locked and the password will expire.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PasswordSecretKeyRef *SecretKeySelector `json:"passwordSecretKeyRef,omitempty"`
	// TLS defines the PKI to be used with MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	TLS *TLS `json:"tls,omitempty"`
	// Connection defines a template to configure the Connection object.
	// This Connection provides the initial User access to the initial Database.
	// It will make use of the Service to route network traffic to all Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Connection *ConnectionTemplate `json:"connection,omitempty" webhook:"inmutable"`
}

// ExternalMariaDBStatus defines the observed state of MariaDB
type ExternalMariaDBStatus struct {
	// Conditions for the ExternalMariadb object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Version of the external MariaDB server
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Version string `json:"version,omitempty"`
	// Is Galera cluster enabled on that MariaDB server
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	IsGaleraEnabled bool `json:"isGaleraEnabled,omitempty"`
}

// SetCondition sets a status condition to ExternalMariaDB
func (s *ExternalMariaDBStatus) SetCondition(condition metav1.Condition) {
	if s.Conditions == nil {
		s.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&s.Conditions, condition)
}

// SetCondition sets a status condition to MariaDB
func (s *ExternalMariaDBStatus) SetVersion(version string) {
	s.Version = version
}

// IsHAEnabled indicates whether the MariaDB instance has Galera enabled
func (m *ExternalMariaDB) IsGaleraEnabled() bool {
	return m.Status.IsGaleraEnabled
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=emdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{ExternalMariaDB,v1alpha1},{Connection,v1alpha1},{ConfigMap,v1},{Secret,v1}}

// ExternalMariaDB is the Schema for the external MariaDBs API. It is used to define external MariaDB server.
type ExternalMariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalMariaDBSpec   `json:"spec"`
	Status ExternalMariaDBStatus `json:"status,omitempty"`
}

// nolint:gocyclo
// SetDefaults sets reasonable defaults.
func (m *ExternalMariaDB) SetDefaults(env *environment.OperatorEnv) error {
	if m.Spec.Port == 0 {
		m.Spec.Port = 3306
	}

	return nil
}

// IsReady indicates whether the External MariaDB instance is ready
func (m *ExternalMariaDB) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReady)
}

// Get image pull policy
func (m *ExternalMariaDB) GetImagePullPolicy() corev1.PullPolicy {
	return m.Spec.ImagePullPolicy
}

// Get image pull secrets
func (m *ExternalMariaDB) GetImagePullSecrets() []LocalObjectReference {
	return m.Spec.ImagePullSecrets
}

// Get image
func (m *ExternalMariaDB) GetImage() string {
	if image := m.Spec.Image; image != "" {
		return image
	}
	version := ptr.Deref(m.Spec.Version, m.Status.Version)
	// By default, ExternalMariaDB uses official MariaDB images (publicly available) for the Backups.
	// This can be overridden by setting the Image field.
	return fmt.Sprintf("mariadb:%s", version)
}

// IsTLSRequired indicates whether TLS is enabled and must be enforced for all connections.
func (m *ExternalMariaDB) IsTLSRequired() bool {
	return false // ExternalMariaDB does not make use of this, as it is a internal server setting
}

// IsTLSEnabled indicates whether TLS is enabled
func (m *ExternalMariaDB) IsTLSEnabled() bool {
	return ptr.Deref(m.Spec.TLS, TLS{}).Enabled
}

// Get MariaDB hostname
func (m *ExternalMariaDB) GetHost() string {
	return m.Spec.Host
}

// Get MariaDB port
func (m *ExternalMariaDB) GetPort() int32 {
	return m.Spec.Port
}

// Get MariaDB replicas
func (m *ExternalMariaDB) GetReplicas() int32 {
	return 0 // ExternalMariaDB does not make use of this
}

// Get MariaDB Superuser name
func (m *ExternalMariaDB) GetSUName() string {
	return ptr.Deref(m.Spec.Username, "root")
}

// Get MariaDB Superuser credentials
func (m *ExternalMariaDB) GetSUCredential() *SecretKeySelector {
	return m.Spec.PasswordSecretKeyRef
}

// +kubebuilder:object:root=true

// External MariaDBList contains a list of ExternalMariaDB
type ExternalMariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalMariaDB `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *ExternalMariaDBList) ListItems() []client.Object {
	items := make([]client.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&ExternalMariaDB{}, &ExternalMariaDBList{})
}
