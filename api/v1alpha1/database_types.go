package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseSpec defines the desired state of Database
type DatabaseSpec struct {
	// SQLTemplate defines templates to configure SQL objects.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SQLTemplate `json:",inline"`
	// MariaDBRef is a reference to a MariaDB object.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// CharacterSet to use in the Database.
	// +optional
	// +kubebuilder:default=utf8
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	CharacterSet string `json:"characterSet,omitempty" webhook:"inmutable"`
	// Collate to use in the Database.
	// +optional
	// +kubebuilder:default=utf8_general_ci
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Collate string `json:"collate,omitempty" webhook:"inmutable"`
	// Name overrides the default Database name provided by metadata.name.
	// +optional
	// +kubebuilder:validation:MaxLength=80
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name,omitempty" webhook:"inmutable"`
}

// DatabaseStatus defines the observed state of Database
type DatabaseStatus struct {
	// Conditions for the Database object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (d *DatabaseStatus) SetCondition(condition metav1.Condition) {
	if d.Conditions == nil {
		d.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&d.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=dmdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="CharSet",type="string",JSONPath=".spec.characterSet"
// +kubebuilder:printcolumn:name="Collate",type="string",JSONPath=".spec.collate"
// +kubebuilder:printcolumn:name="MariaDB",type="string",JSONPath=".spec.mariaDbRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".spec.name"
// +operator-sdk:csv:customresourcedefinitions:resources={{Database,v1alpha1}}

// Database is the Schema for the databases API. It is used to define a logical database as if you were running a 'CREATE DATABASE' statement.
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec,omitempty"`
	Status DatabaseStatus `json:"status,omitempty"`
}

func (d *Database) DatabaseNameOrDefault() string {
	if d.Spec.Name != "" {
		return d.Spec.Name
	}
	return d.Name
}

func (d *Database) IsBeingDeleted() bool {
	return !d.DeletionTimestamp.IsZero()
}

func (d *Database) IsReady() bool {
	return meta.IsStatusConditionTrue(d.Status.Conditions, ConditionTypeReady)
}

func (d *Database) MariaDBRef() *MariaDBRef {
	return &d.Spec.MariaDBRef
}

func (d *Database) RequeueInterval() *metav1.Duration {
	return d.Spec.RequeueInterval
}

func (d *Database) RetryInterval() *metav1.Duration {
	return d.Spec.RetryInterval
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
