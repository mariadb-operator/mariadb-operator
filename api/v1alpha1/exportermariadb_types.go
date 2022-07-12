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

type Exporter struct {
	// +kubebuilder:validation:Required
	Image            Image                         `json:"image"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	Resources        *corev1.ResourceRequirements  `json:"resources,omitempty"`
}

// ExporterMariaDBSpec defines the desired state of ExporterMariaDB
type ExporterMariaDBSpec struct {
	// +kubebuilder:validation:Required
	MariaDBRef corev1.LocalObjectReference `json:"mariaDbRef"`
	// +kubebuilder:validation:Required
	Exporter `json:",inline"`
}

// ExporterMariaDBStatus defines the observed state of ExporterMariaDB
type ExporterMariaDBStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (m *ExporterMariaDBStatus) SetCondition(condition metav1.Condition) {
	if m.Conditions == nil {
		m.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&m.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=emdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="MariaDB",type="string",JSONPath=".spec.mariaDbRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ExporterMariaDB is the Schema for the exportermariadbs API
type ExporterMariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExporterMariaDBSpec   `json:"spec,omitempty"`
	Status ExporterMariaDBStatus `json:"status,omitempty"`
}

func (e *ExporterMariaDB) IsReady() bool {
	return meta.IsStatusConditionTrue(e.Status.Conditions, ConditionTypeReady)
}

// +kubebuilder:object:root=true

// ExporterMariaDBList contains a list of ExporterMariaDB
type ExporterMariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExporterMariaDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExporterMariaDB{}, &ExporterMariaDBList{})
}
