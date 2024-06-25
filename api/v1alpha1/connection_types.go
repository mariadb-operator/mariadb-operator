package v1alpha1

import (
	"errors"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	defaultConnSecretKey = "dsn"
)

// ConnectionRefs contains the resolved references of a Connection.
type ConnectionRefs struct {
	MariaDB  *MariaDB
	MaxScale *MaxScale
}

// Host returns the host address to connect to.
func (r *ConnectionRefs) Host(c *Connection) (*string, error) {
	objMeta, err := r.objectMeta()
	if err != nil {
		return nil, err
	}
	if c.Spec.ServiceName != nil {
		svcMeta := metav1.ObjectMeta{
			Name:      *c.Spec.ServiceName,
			Namespace: objMeta.Namespace,
		}
		return ptr.To(statefulset.ServiceFQDN(svcMeta)), nil
	}
	return ptr.To(statefulset.ServiceFQDN(*objMeta)), nil
}

// Port returns the port to connect to.
func (r *ConnectionRefs) Port() (*int32, error) {
	if r.MariaDB != nil {
		return &r.MariaDB.Spec.Port, nil
	}
	if r.MaxScale != nil {
		return r.MaxScale.DefaultPort()
	}
	return nil, errors.New("port not found")
}

func (r *ConnectionRefs) objectMeta() (*metav1.ObjectMeta, error) {
	if r.MariaDB != nil {
		return &r.MariaDB.ObjectMeta, nil
	}
	if r.MaxScale != nil {
		return &r.MaxScale.ObjectMeta, nil
	}
	return nil, errors.New("references not found")
}

// ConnectionSpec defines the desired state of Connection
type ConnectionSpec struct {
	// ContainerTemplate defines templates to configure Container objects.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ConnectionTemplate `json:",inline"`
	// MariaDBRef is a reference to the MariaDB to connect to. Either MariaDBRef or MaxScaleRef must be provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef *MariaDBRef `json:"mariaDbRef,omitempty"`
	// MaxScaleRef is a reference to the MaxScale to connect to. Either MariaDBRef or MaxScaleRef must be provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MaxScaleRef *corev1.ObjectReference `json:"maxScaleRef,omitempty"`
	// Username to use for configuring the Connection.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Username string `json:"username"`
	// PasswordSecretKeyRef is a reference to the password to use for configuring the Connection.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PasswordSecretKeyRef corev1.SecretKeySelector `json:"passwordSecretKeyRef"`
	// Host to connect to. If not provided, it defaults to the MariaDB host or to the MaxScale host.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	Host string `json:"host,omitempty"`
	// Database to use when configuring the Connection.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Database *string `json:"database,omitempty"`
}

// ConnectionStatus defines the observed state of Connection
type ConnectionStatus struct {
	// Conditions for the Connection object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (c *ConnectionStatus) SetCondition(condition metav1.Condition) {
	if c.Conditions == nil {
		c.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&c.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=cmdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Secret",type="string",JSONPath=".spec.secretName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{Connection,v1alpha1},{Secret,v1}}

// Connection is the Schema for the connections API. It is used to configure connection strings for the applications connecting to MariaDB.
type Connection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConnectionSpec   `json:"spec,omitempty"`
	Status ConnectionStatus `json:"status,omitempty"`
}

func (c *Connection) IsReady() bool {
	return meta.IsStatusConditionTrue(c.Status.Conditions, ConditionTypeReady)
}

func (c *Connection) SetDefaults(refs *ConnectionRefs) error {
	if c.Spec.Host == "" {
		host, err := refs.Host(c)
		if err != nil {
			return err
		}
		c.Spec.Host = *host
	}
	if c.Spec.Port == 0 {
		port, err := refs.Port()
		if err != nil {
			return err
		}
		c.Spec.Port = *port
	}
	if c.Spec.SecretName == nil {
		c.Spec.SecretName = &c.Name
	}
	if c.Spec.SecretTemplate == nil {
		c.Spec.SecretTemplate = &SecretTemplate{
			Key: &defaultConnSecretKey,
		}
	}
	return nil
}

func (c *Connection) SecretName() string {
	if c.Spec.SecretName != nil {
		return *c.Spec.SecretName
	}
	return c.Name
}

func (c *Connection) SecretKey() string {
	if c.Spec.SecretTemplate.Key != nil {
		return *c.Spec.SecretTemplate.Key
	}
	return defaultConnSecretKey
}

// ConnectionPasswordSecretFieldPath is the path related to the password Secret field.
const ConnectionPasswordSecretFieldPath = ".spec.passwordSecretKeyRef.name"

// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (c *Connection) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case ConnectionPasswordSecretFieldPath:
		return func(obj client.Object) []string {
			connection, ok := obj.(*Connection)
			if !ok {
				return nil
			}
			if connection.Spec.PasswordSecretKeyRef.LocalObjectReference.Name != "" {
				return []string{connection.Spec.PasswordSecretKeyRef.LocalObjectReference.Name}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

//+kubebuilder:object:root=true

// ConnectionList contains a list of Connection
type ConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Connection `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *ConnectionList) ListItems() []client.Object {
	items := make([]client.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&Connection{}, &ConnectionList{})
}
