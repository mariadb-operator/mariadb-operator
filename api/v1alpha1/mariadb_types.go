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
	"errors"

	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type InheritMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type Exporter struct {
	ContainerTemplate `json:",inline"`
	// +kubebuilder:default=9104
	Port int32 `json:"port,omitempty"`
}

type ServiceMonitor struct {
	// +kubebuilder:validation:Required
	PrometheusRelease string `json:"prometheusRelease"`
	Interval          string `json:"interval,omitempty"`
	ScrapeTimeout     string `json:"scrapeTimeout,omitempty"`
}

type Metrics struct {
	// +kubebuilder:validation:Required
	Exporter Exporter `json:"exporter"`
	// +kubebuilder:validation:Required
	ServiceMonitor ServiceMonitor `json:"serviceMonitor"`
}

type PodDisruptionBudget struct {
	MinAvailable   *intstr.IntOrString `json:"minAvailable,omitempty"`
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

func (p *PodDisruptionBudget) Validate() error {
	if p.MinAvailable != nil && p.MaxUnavailable == nil {
		return nil
	}
	if p.MinAvailable == nil && p.MaxUnavailable != nil {
		return nil
	}
	return errors.New("either minAvailable or maxUnavailable must be specified")
}

type ServiceTemplate struct {
	Type        corev1.ServiceType `json:"type,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty"`
	ExternalTrafficPolicy *string `json:"externalTrafficPolicy,omitempty"`
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`
	LoadBalancerIp *string `json:"loadBalancerIp,omitempty"`
}

// MariaDBSpec defines the desired state of MariaDB
type MariaDBSpec struct {
	ContainerTemplate `json:",inline"`
	PodTemplate       `json:",inline"`

	InheritMetadata *InheritMetadata `json:"inheritMetadata,omitempty"`
	// +kubebuilder:validation:Required
	RootPasswordSecretKeyRef corev1.SecretKeySelector `json:"rootPasswordSecretKeyRef" webhook:"inmutable"`

	Database             *string                   `json:"database,omitempty" webhook:"inmutable"`
	Username             *string                   `json:"username,omitempty" webhook:"inmutable"`
	PasswordSecretKeyRef *corev1.SecretKeySelector `json:"passwordSecretKeyRef,omitempty" webhook:"inmutable"`

	MyCnf                *string                      `json:"myCnf,omitempty" webhook:"inmutable"`
	MyCnfConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"myCnfConfigMapKeyRef,omitempty" webhook:"inmutableinit"`

	BootstrapFrom *RestoreSource `json:"bootstrapFrom,omitempty"`

	Metrics *Metrics `json:"metrics,omitempty"`

	Replication *Replication `json:"replication,omitempty"`

	Galera *Galera `json:"galera,omitempty"`
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`
	// +kubebuilder:default=3306
	Port int32 `json:"port,omitempty"`
	// +kubebuilder:validation:Required
	VolumeClaimTemplate VolumeClaimTemplate `json:"volumeClaimTemplate" webhook:"inmutable"`

	PodDisruptionBudget *PodDisruptionBudget `json:"podDisruptionBudget,omitempty"`

	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	Service    *ServiceTemplate    `json:"service,omitempty"`
	Connection *ConnectionTemplate `json:"connection,omitempty" webhook:"inmutable"`

	PrimaryService    *ServiceTemplate    `json:"primaryService,omitempty"`
	PrimaryConnection *ConnectionTemplate `json:"primaryConnection,omitempty" webhook:"inmutable"`

	SecondaryService    *ServiceTemplate    `json:"secondaryService,omitempty"`
	SecondaryConnection *ConnectionTemplate `json:"secondaryConnection,omitempty" webhook:"inmutable"`
}

// MariaDBStatus defines the observed state of MariaDB
type MariaDBStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	CurrentPrimaryPodIndex *int    `json:"currentPrimaryPodIndex,omitempty"`
	CurrentPrimary         *string `json:"currentPrimary,omitempty"`

	GaleraRecovery *GaleraRecoveryStatus `json:"galeraRecovery,omitempty"`
}

func (s *MariaDBStatus) SetCondition(condition metav1.Condition) {
	if s.Conditions == nil {
		s.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&s.Conditions, condition)
}

// UpdateCurrentPrimary updates the current primary status.
func (s *MariaDBStatus) UpdateCurrentPrimary(mariadb *MariaDB, index int) {
	s.CurrentPrimaryPodIndex = &index
	currentPrimary := statefulset.PodName(mariadb.ObjectMeta, index)
	s.CurrentPrimary = &currentPrimary
}

// FillWithDefaults fills the current MariaDBStatus object with defaults.
func (s *MariaDBStatus) FillWithDefaults(mariadb *MariaDB) {
	if s.CurrentPrimaryPodIndex == nil {
		index := 0
		s.CurrentPrimaryPodIndex = &index
	}
	if s.CurrentPrimary == nil {
		currentPrimary := statefulset.PodName(mariadb.ObjectMeta, *s.CurrentPrimaryPodIndex)
		s.CurrentPrimary = &currentPrimary
	}
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=mdb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Primary Pod",type="string",JSONPath=".status.currentPrimary"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MariaDB is the Schema for the mariadbs API
type MariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MariaDBSpec   `json:"spec"`
	Status MariaDBStatus `json:"status,omitempty"`
}

func (m *MariaDB) Replication() Replication {
	if m.Spec.Replication == nil {
		m.Spec.Replication = &Replication{}
	}
	m.Spec.Replication.FillWithDefaults()
	return *m.Spec.Replication
}

func (m *MariaDB) Galera() Galera {
	if m.Spec.Galera == nil {
		m.Spec.Galera = &Galera{}
	}
	m.Spec.Galera.FillWithDefaults()
	return *m.Spec.Galera
}

func (m *MariaDB) IsHAEnabled() bool {
	return m.Replication().Enabled || m.Galera().Enabled
}

func (m *MariaDB) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReady)
}

func (m *MariaDB) IsRestoringBackup() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeBackupRestored)
}

func (m *MariaDB) HasRestoredBackup() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeBackupRestored)
}

// +kubebuilder:object:root=true

// MariaDBList contains a list of MariaDB
type MariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MariaDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MariaDB{}, &MariaDBList{})
}
