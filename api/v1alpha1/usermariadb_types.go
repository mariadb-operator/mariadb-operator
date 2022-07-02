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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// UserMariaDBSpec defines the desired state of UserMariaDB
type UserMariaDBSpec struct {
	// +kubebuilder:validation:Required
	MariaDBRef corev1.LocalObjectReference `json:"mariaDbRef"`
	// +kubebuilder:validation:Required
	IdentifiedBySecretKeyRef corev1.SecretKeySelector `json:"identifiedBySecretKeyRef"`
	// +kubebuilder:default=10
	MaxUserConnections int32 `json:"maxUserConnections,omitempty"`
}

// UserMariaDBStatus defines the observed state of UserMariaDB
type UserMariaDBStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (r *UserMariaDBStatus) AddCondition(condition metav1.Condition) {
	if r.Conditions == nil {
		r.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&r.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=umdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Username",type="string",JSONPath=".spec.username"
// +kubebuilder:printcolumn:name="MaxConnections",type="string",JSONPath=".spec.maxUserConnections"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// UserMariaDB is the Schema for the usermariadbs API
type UserMariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserMariaDBSpec   `json:"spec,omitempty"`
	Status UserMariaDBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserMariaDBList contains a list of UserMariaDB
type UserMariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserMariaDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UserMariaDB{}, &UserMariaDBList{})
}
