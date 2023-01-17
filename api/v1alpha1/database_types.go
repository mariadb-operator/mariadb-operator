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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseSpec defines the desired state of Database
type DatabaseSpec struct {
	// +kubebuilder:validation:Required
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// +kubebuilder:default=utf8
	CharacterSet string `json:"characterSet,omitempty" webhook:"inmutable"`
	// +kubebuilder:default=utf8_general_ci
	Collate string `json:"collate,omitempty" webhook:"inmutable"`
}

// DatabaseStatus defines the observed state of Database
type DatabaseStatus struct {
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
// +kubebuilder:resource:shortName=db
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="CharSet",type="string",JSONPath=".spec.characterSet"
// +kubebuilder:printcolumn:name="Collate",type="string",JSONPath=".spec.collate"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Database is the Schema for the databases API
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec,omitempty"`
	Status DatabaseStatus `json:"status,omitempty"`
}

func (d *Database) IsBeingDeleted() bool {
	return !d.DeletionTimestamp.IsZero()
}

func (m *Database) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReady)
}

func (m *Database) Meta() metav1.ObjectMeta {
	return m.ObjectMeta
}

func (m *Database) MariaDBRef() *MariaDBRef {
	return &m.Spec.MariaDBRef
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
