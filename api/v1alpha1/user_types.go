package v1alpha1

import (
	"errors"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TLSRequirements specifies TLS requirements for the user to connect. See: https://mariadb.com/kb/en/securing-connections-for-client-and-server/#requiring-tls.
type TLSRequirements struct {
	// SSL indicates that the user must connect via TLS.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SSL *bool `json:"ssl,omitempty"`
	// X509 indicates that the user must provide a valid x509 certificate to connect.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	X509 *bool `json:"x509,omitempty"`
	// Issuer indicates that the TLS certificate provided by the user must be issued by a specific issuer.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Issuer *string `json:"issuer,omitempty"`
	// Subject indicates that the TLS certificate provided by the user must have a specific subject.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Subject *string `json:"subject,omitempty"`
}

// Validate ensures that TLSRequirements provides legit options.
func (u *TLSRequirements) Validate() error {
	// see: https://mariadb.com/kb/en/securing-connections-for-client-and-server/#requiring-tls
	count := 0
	if u.SSL != nil && *u.SSL {
		count++
	}
	if u.X509 != nil && *u.X509 {
		count++
	}
	if (u.Issuer != nil && *u.Issuer != "") || (u.Subject != nil && *u.Subject != "") {
		count++
	}
	if count > 1 {
		return errors.New("only one of [SSL, X509, (Issuer, Subject)] can be set at a time")
	}
	if count == 0 {
		return errors.New("at least one field [SSL, X509, (Issuer, Subject)] must be set")
	}
	return nil
}

// UserSpec defines the desired state of User
type UserSpec struct {
	// SQLTemplate defines templates to configure SQL objects.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SQLTemplate `json:",inline"`
	// MariaDBRef is a reference to a MariaDB object.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// PasswordSecretKeyRef is a reference to the password to be used by the User.
	// If not provided, the account will be locked and the password will expire.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PasswordSecretKeyRef *SecretKeySelector `json:"passwordSecretKeyRef,omitempty"`
	// PasswordHashSecretKeyRef is a reference to the password hash to be used by the User.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password hash.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PasswordHashSecretKeyRef *SecretKeySelector `json:"passwordHashSecretKeyRef,omitempty"`
	// PasswordPlugin is a reference to the password plugin and arguments to be used by the User.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PasswordPlugin PasswordPlugin `json:"passwordPlugin,omitempty"`
	// Require specifies TLS requirements for the user to connect. See: https://mariadb.com/kb/en/securing-connections-for-client-and-server/#requiring-tls.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Require *TLSRequirements `json:"require,omitempty"`
	// ClientCertificate configures whether the operator issues a dedicated TLS client certificate for this User.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientCertificate *UserClientCertificate `json:"clientCertificate,omitempty"`
	// MaxUserConnections defines the maximum number of simultaneous connections that the User can establish.
	// +optional
	// +kubebuilder:default=10
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	MaxUserConnections int32 `json:"maxUserConnections,omitempty"`
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

// UserClientCertificate configures whether the operator issues a dedicated TLS client certificate for a User.
type UserClientCertificate struct {
	// Enabled determines whether the operator issues a TLS client certificate for this User, signed by the same
	// CA used by the referenced MariaDB (spec.tls.clientCASecretRef / spec.tls.clientCertIssuerRef, or the
	// operator-managed CA otherwise). It requires the referenced MariaDB to have TLS enabled (spec.tls.enabled: true).
	// It is disabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
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

// User is the Schema for the users API.  It is used to define grants as if you were running a 'CREATE USER' statement.
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

func (u *User) AccountName() string {
	return fmt.Sprintf("'%s'@'%s'", u.UsernameOrDefault(), u.HostnameOrDefault())
}

func (u *User) UsernameOrDefault() string {
	if u.Spec.Name != "" {
		return u.Spec.Name
	}
	return u.Name
}

func (u *User) HostnameOrDefault() string {
	if u.Spec.Host != "" {
		return u.Spec.Host
	}
	return "%"
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

func (u *User) RequeueInterval() *metav1.Duration {
	return u.Spec.RequeueInterval
}

func (u *User) RetryInterval() *metav1.Duration {
	return u.Spec.RetryInterval
}

func (u *User) CleanupPolicy() *CleanupPolicy {
	return u.Spec.CleanupPolicy
}

// IsClientCertificateEnabled indicates whether the operator should issue a dedicated TLS client certificate
// for this User (spec.clientCertificate.enabled).
func (u *User) IsClientCertificateEnabled() bool {
	return u.Spec.ClientCertificate != nil && u.Spec.ClientCertificate.Enabled
}

// TLSClientCertSecretKey defines the key for the User's TLS client cert.
func (u *User) TLSClientCertSecretKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-client-cert", u.Name),
		Namespace: u.Namespace,
	}
}

// TLSClientNames are the X.509 subject common names for the User's TLS client cert.
func (u *User) TLSClientNames() []string {
	return []string{fmt.Sprintf("%s-client", u.Name)}
}

// ClientCertCollidesWith indicates whether the User's TLS client certificate would clash with the one of the MariaDB:
// both are signed by the same client CA, so a User with the same common name would be able to impersonate the
// operator-managed users, and one with the same Secret would overwrite the operator's certificate.
func (u *User) ClientCertCollidesWith(mdb *MariaDB) bool {
	return slices.ContainsFunc(u.TLSClientNames(), func(name string) bool {
		return slices.Contains(mdb.TLSClientNames(), name)
	}) || u.TLSClientCertSecretKey() == mdb.TLSClientCertSecretKey()
}

// +kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *UserList) ListItems() []client.Object {
	items := make([]client.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}
