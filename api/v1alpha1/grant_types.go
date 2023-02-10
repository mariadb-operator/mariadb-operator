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

// GrantSpec defines the desired state of Grant
type GrantSpec struct {
	// +kubebuilder:validation:Required
	MariaDBRef MariaDBRef `json:"mariaDbRef" webhook:"inmutable"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Privileges []string `json:"privileges" webhook:"inmutable"`
	// +kubebuilder:default=*
	Database string `json:"database,omitempty" webhook:"inmutable"`
	// +kubebuilder:default=*
	Table string `json:"table,omitempty" webhook:"inmutable"`
	// +kubebuilder:validation:Required
	Username string `json:"username" webhook:"inmutable"`
	// +kubebuilder:default=false
	GrantOption bool `json:"grantOption,omitempty" webhook:"inmutable"`
}

// GrantStatus defines the observed state of Grant
type GrantStatus struct {
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
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Grant is the Schema for the grants API
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

func (g *Grant) Meta() metav1.ObjectMeta {
	return g.ObjectMeta
}

func (g *Grant) MariaDBRef() *MariaDBRef {
	return &g.Spec.MariaDBRef
}

//+kubebuilder:object:root=true

// GrantList contains a list of Grant
type GrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Grant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Grant{}, &GrantList{})
}
