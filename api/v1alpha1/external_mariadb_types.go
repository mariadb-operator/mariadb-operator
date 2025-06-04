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

const (
	// ExternalMariaDBKind
	ExternalMariaDBKind = "ExternalMariaDB"
)

// ExternalMariaDBSpec defines the desired state of an External MariaDB
type ExternalMariaDBSpec struct {

	// Hostname of the external MariaDB service.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Host string `json:"host"`
	// Port of the external MariaDB.
	// +optional
	// +kubebuilder:default=3306
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	Port int32 `json:"port,omitempty"`
	// Username is the username to connect to the external MariaDB.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Username *string `json:"username"`
	// PasswordSecretKeyRef is a reference to the password to be used by the User.
	// If not provided, the account will be locked and the password will expire.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PasswordSecretKeyRef *SecretKeySelector `json:"passwordSecretKeyRef,omitempty"`
	// InheritMetadata defines the metadata to be inherited by children resources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InheritMetadata *Metadata `json:"inheritMetadata,omitempty"`
	// TLS defines the PKI to be used with MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	TLS *TLS `json:"tls,omitempty"`
	// Connection defines a template to configure the general Connection object.
	// This Connection provides the initial User access to the initial Database.
	// It will make use of the Service to route network traffic to all Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Connection *ConnectionTemplate `json:"connection,omitempty" webhook:"inmutable"`
	// External MariaDB Version
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Version *string `json:"version,omitempty"`
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
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{ExternalMariaDB,v1alpha1},{MariaDB,v1alpha1},{MaxScale,v1alpha1},{Connection,v1alpha1},{Restore,v1alpha1},{User,v1alpha1},{Grant,v1alpha1},{ConfigMap,v1},{Service,v1},{Secret,v1},{Event,v1},{ServiceAccount,v1},{StatefulSet,v1},{Deployment,v1},{Job,v1},{PodDisruptionBudget,v1},{Role,v1},{RoleBinding,v1},{ClusterRoleBinding,v1}}

// ExternalMariaDB is the Schema for the external mariadbs API. It is used to define external MariaDB server.
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
	return corev1.PullIfNotPresent
}

// Get image pull secrets
func (m *ExternalMariaDB) GetImagePullSecrets() []LocalObjectReference {
	return nil // ExternalMariaDB uses official MariaDB images (publicly available) for the Backups
}

// Get image
func (m *ExternalMariaDB) GetImage() string {
	var version string
	if m.Spec.Version != nil {
		version = *m.Spec.Version
	} else {
		version = m.Status.Version
	}
	return fmt.Sprintf("mariadb:%s", version) // ExternalMariaDB uses official MariaDB images (publicly available) for the Backups
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
	return 1
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
