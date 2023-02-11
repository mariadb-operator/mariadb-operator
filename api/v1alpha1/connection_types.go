/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	defaultConnSecretKey = "dsn"
)

// ConnectionSpec defines the desired state of Connection
type ConnectionSpec struct {
	ConnectionTemplate `json:",inline"`
	// +kubebuilder:validation:Required
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// +kubebuilder:validation:Required
	Username string `json:"username" webhook:"inmutable"`
	// +kubebuilder:validation:Required
	PasswordSecretKeyRef corev1.SecretKeySelector `json:"passwordSecretKeyRef" webhook:"inmutable"`

	Database *string `json:"database,omitempty" webhook:"inmutable"`
}

// ConnectionStatus defines the observed state of Connection
type ConnectionStatus struct {
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
// +kubebuilder:printcolumn:name="MariaDB",type="string",JSONPath=".spec.mariaDbRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Connection is the Schema for the connections API
type Connection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConnectionSpec   `json:"spec,omitempty"`
	Status ConnectionStatus `json:"status,omitempty"`
}

func (c *Connection) IsReady() bool {
	return meta.IsStatusConditionTrue(c.Status.Conditions, ConditionTypeReady)
}

func (c *Connection) IsInit() bool {
	return c.Spec.SecretName != nil && c.Spec.SecretTemplate != nil
}

func (c *Connection) Init() {
	if c.Spec.SecretName == nil {
		c.Spec.SecretName = &c.Name
	}
	if c.Spec.SecretTemplate == nil {
		c.Spec.SecretTemplate = &SecretTemplate{
			Key: &defaultConnSecretKey,
		}
	}
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

//+kubebuilder:object:root=true

// ConnectionList contains a list of Connection
type ConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Connection `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Connection{}, &ConnectionList{})
}
