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

type Service struct {
	Type        corev1.ServiceType `json:"type,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty"`
}

// MariaDBSpec defines the desired state of MariaDB
type MariaDBSpec struct {
	ContainerTemplate `json:",inline"`

	InheritMetadata *InheritMetadata `json:"inheritMetadata,omitempty"`
	// +kubebuilder:validation:Required
	RootPasswordSecretKeyRef corev1.SecretKeySelector `json:"rootPasswordSecretKeyRef" webhook:"inmutable"`

	Database             *string                   `json:"database,omitempty" webhook:"inmutable"`
	Username             *string                   `json:"username,omitempty" webhook:"inmutable"`
	PasswordSecretKeyRef *corev1.SecretKeySelector `json:"passwordSecretKeyRef,omitempty" webhook:"inmutable"`
	Connection           *ConnectionTemplate       `json:"connection,omitempty" webhook:"inmutable"`

	MyCnf                *string                      `json:"myCnf,omitempty" webhook:"inmutable"`
	MyCnfConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"myCnfConfigMapKeyRef,omitempty" webhook:"inmutableinit"`

	BootstrapFrom *RestoreSource `json:"bootstrapFrom,omitempty"`

	Metrics *Metrics `json:"metrics,omitempty"`

	Replication *Replication `json:"replication,omitempty"`

	Galera *Galera `json:"galera,omitempty"`
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" webhook:"inmutable"`
	// +kubebuilder:default=3306
	Port int32 `json:"port,omitempty"`
	// +kubebuilder:validation:Required
	VolumeClaimTemplate corev1.PersistentVolumeClaimSpec `json:"volumeClaimTemplate" webhook:"inmutable"`
	Volumes             []corev1.Volume                  `json:"volumes,omitempty" webhook:"inmutable"`

	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`

	PodDisruptionBudget *PodDisruptionBudget `json:"podDisruptionBudget,omitempty"`

	Service *Service `json:"service,omitempty"`
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
