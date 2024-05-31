package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GrantSpec defines the desired state of Grant
type GrantSpec struct {
	// SQLTemplate defines templates to configure SQL objects.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SQLTemplate `json:",inline"`
	// MariaDBRef is a reference to a MariaDB object.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// Privileges to use in the Grant.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Privileges []string `json:"privileges" webhook:"inmutable"`
	// Database to use in the Grant.
	// +optional
	// +kubebuilder:default=*
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Database string `json:"database,omitempty" webhook:"inmutable"`
	// Table to use in the Grant.
	// +optional
	// +kubebuilder:default=*
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Table string `json:"table,omitempty" webhook:"inmutable"`
	// Username to use in the Grant.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Username string `json:"username" webhook:"inmutable"`
	// Host to use in the Grant. It can be localhost, an IP or '%'.
	// +optional
	// +kubebuilder:MaxLength=255
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Host *string `json:"host,omitempty" webhook:"inmutable"`
	// GrantOption to use in the Grant.
	// +optional
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	GrantOption bool `json:"grantOption,omitempty" webhook:"inmutable"`
}

// GrantStatus defines the observed state of Grant
type GrantStatus struct {
	// Conditions for the Grant object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (g *GrantStatus) SetCondition(condition metav1.Condition) {
	if g.Conditions == nil {
		g.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&g.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=gmdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Database",type="string",JSONPath=".spec.database"
// +kubebuilder:printcolumn:name="Table",type="string",JSONPath=".spec.table"
// +kubebuilder:printcolumn:name="Username",type="string",JSONPath=".spec.username"
// +kubebuilder:printcolumn:name="GrantOpt",type="string",JSONPath=".spec.grantOption"
// +kubebuilder:printcolumn:name="MariaDB",type="string",JSONPath=".spec.mariaDbRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{Grant,v1alpha1}}

// Grant is the Schema for the grants API. It is used to define grants as if you were running a 'GRANT' statement.
type Grant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrantSpec   `json:"spec,omitempty"`
	Status GrantStatus `json:"status,omitempty"`
}

func (g *Grant) IsBeingDeleted() bool {
	return !g.DeletionTimestamp.IsZero()
}

func (m *Grant) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReady)
}

func (g *Grant) MariaDBRef() *MariaDBRef {
	return &g.Spec.MariaDBRef
}

func (d *Grant) RequeueInterval() *metav1.Duration {
	return d.Spec.RequeueInterval
}

func (g *Grant) RetryInterval() *metav1.Duration {
	return g.Spec.RetryInterval
}

func (g *Grant) AccountName() string {
	return fmt.Sprintf("'%s'@'%s'", g.Spec.Username, g.HostnameOrDefault())
}

func (g *Grant) HostnameOrDefault() string {
	if g.Spec.Host != nil && *g.Spec.Host != "" {
		return *g.Spec.Host
	}
	return "%"
}

// GrantUsernameFieldPath is the path related to the username field.
const GrantUsernameFieldPath = ".spec.username"

// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (g *Grant) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case GrantUsernameFieldPath:
		return func(obj client.Object) []string {
			grant, ok := obj.(*Grant)
			if !ok {
				return nil
			}
			if grant.Spec.Username != "" {
				return []string{grant.Spec.Username}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

//+kubebuilder:object:root=true

// GrantList contains a list of Grant
type GrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Grant `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *GrantList) ListItems() []ctrlclient.Object {
	items := make([]ctrlclient.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&Grant{}, &GrantList{})
}
