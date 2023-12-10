package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserSpec defines the desired state of User
type UserSpec struct {
	// SQLTemplate defines templates to configure SQL objects.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SQLTemplate `json:",inline"`
	// MariaDBRef is a reference to a MariaDB object.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// PasswordSecretKeyRef is a reference to the password to be used by the User.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PasswordSecretKeyRef corev1.SecretKeySelector `json:"passwordSecretKeyRef" webhook:"inmutable"`
	// MaxUserConnections defines the maximum number of connections that the User can have.
	// +optional
	// +kubebuilder:default=10
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	MaxUserConnections int32 `json:"maxUserConnections,omitempty" webhook:"inmutable"`
	// Name overrides the default name provided by metadata.name.
	// +optional
	// +kubebuilder:validation:MaxLength=80
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name,omitempty" webhook:"inmutable"`
	// Host related to the User.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Host string `json:"host,omitempty" webhook:"inmutable"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// Conditions for the User object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (u *UserStatus) SetCondition(condition metav1.Condition) {
	if u.Conditions == nil {
		u.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&u.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=umdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="MaxConns",type="string",JSONPath=".spec.maxUserConnections"
// +kubebuilder:printcolumn:name="MariaDB",type="string",JSONPath=".spec.mariaDbRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{User,v1alpha1}}

// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

func (u *User) AccountName() string {
	return fmt.Sprintf("'%s'@'%s'", u.usernameOrDefault(), u.hostnameOrDefault())
}

func (u *User) IsBeingDeleted() bool {
	return !u.DeletionTimestamp.IsZero()
}

func (u *User) IsReady() bool {
	return meta.IsStatusConditionTrue(u.Status.Conditions, ConditionTypeReady)
}

func (u *User) MariaDBRef() *MariaDBRef {
	return &u.Spec.MariaDBRef
}

func (u *User) RetryInterval() *metav1.Duration {
	return u.Spec.RetryInterval
}

func (u *User) usernameOrDefault() string {
	if u.Spec.Name != "" {
		return u.Spec.Name
	}
	return u.Name
}

func (u *User) hostnameOrDefault() string {
	if u.Spec.Host != "" {
		return u.Spec.Host
	}
	return "%"
}

// +kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
