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

// MonitorMariaDBSpec defines the desired state of MonitorMariaDB
type MonitorMariaDBSpec struct {
	// +kubebuilder:validation:Required
	MariaDBRef corev1.LocalObjectReference `json:"mariaDbRef"`
	// +kubebuilder:default='10s'
	Interval string `json:"interval,omitempty"`
	// +kubebuilder:default='10s'
	ScrapeTimeout string `json:"scrapeTimeout,omitempty"`
}

// MonitorMariaDBStatus defines the observed state of MonitorMariaDB
type MonitorMariaDBStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (m *MonitorMariaDBStatus) AddCondition(condition metav1.Condition) {
	if m.Conditions == nil {
		m.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&m.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=mmdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="MariaDB",type="string",JSONPath=".spec.mariaDbRef.name"
// +kubebuilder:printcolumn:name="Interval",type="string",JSONPath=".spec.interval"
// +kubebuilder:printcolumn:name="ScrapeTimeout",type="string",JSONPath=".spec.scrapeTimeout"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MonitorMariaDB is the Schema for the monitormariadbs API
type MonitorMariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MonitorMariaDBSpec   `json:"spec,omitempty"`
	Status MonitorMariaDBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MonitorMariaDBList contains a list of MonitorMariaDB
type MonitorMariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MonitorMariaDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MonitorMariaDB{}, &MonitorMariaDBList{})
}
