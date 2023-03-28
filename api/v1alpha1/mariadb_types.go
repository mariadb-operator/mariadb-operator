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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Exporter struct {
	// +kubebuilder:validation:Required
	Image     Image                        `json:"image"`
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
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

type Service struct {
	Type        corev1.ServiceType `json:"type,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty"`
}

type ReplicationMode string

const (
	ReplicationModeAsync    ReplicationMode = "Async"
	ReplicationModeSemiSync ReplicationMode = "SemiSync"
)

func (r ReplicationMode) Validate() error {
	switch r {
	case ReplicationModeAsync, ReplicationModeSemiSync:
		return nil
	default:
		return fmt.Errorf("invalid ReplicationMode: %v", r)
	}
}

type WaitPoint string

const (
	WaitPointAfterSync   WaitPoint = "AfterSync"
	WaitPointAfterCommit WaitPoint = "AfterCommit"
)

func (w WaitPoint) Validate() error {
	switch w {
	case WaitPointAfterSync, WaitPointAfterCommit:
		return nil
	default:
		return fmt.Errorf("invalid WaitPoint: %v", w)
	}
}

func (w WaitPoint) MariaDBFormat() (string, error) {
	switch w {
	case WaitPointAfterSync:
		return "AFTER_SYNC", nil
	case WaitPointAfterCommit:
		return "AFTER_COMMIT", nil
	default:
		return "", fmt.Errorf("invalid WaitPoint: %v", w)
	}
}

type Replication struct {
	Mode ReplicationMode `json:"mode"`

	WaitPoint *WaitPoint `json:"waitPoint,omitempty"`

	PrimaryTimeout *metav1.Duration `json:"primaryTimeout,omitempty"`

	ReplicaRetries *int32 `json:"replicaRetries,omitempty"`
}

func (r *Replication) Validate() error {
	if err := r.Mode.Validate(); err != nil {
		return fmt.Errorf("invalid Replication: %v", err)
	}
	if r.Mode == ReplicationModeAsync {
		return nil
	}
	if r.WaitPoint != nil {
		if err := r.WaitPoint.Validate(); err != nil {
			return fmt.Errorf("invalid WaitPoint: %v", err)
		}
	}
	return nil
}

// MariaDBSpec defines the desired state of MariaDB
type MariaDBSpec struct {
	// +kubebuilder:validation:Required
	RootPasswordSecretKeyRef corev1.SecretKeySelector `json:"rootPasswordSecretKeyRef" webhook:"inmutable"`

	Database             *string                   `json:"database,omitempty" webhook:"inmutable"`
	Username             *string                   `json:"username,omitempty" webhook:"inmutable"`
	PasswordSecretKeyRef *corev1.SecretKeySelector `json:"passwordSecretKeyRef,omitempty" webhook:"inmutable"`
	Connection           *ConnectionTemplate       `json:"connection,omitempty" webhook:"inmutable"`
	// +kubebuilder:validation:Required
	Image            Image                         `json:"image" webhook:"inmutable"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" webhook:"inmutable"`
	// +kubebuilder:default=3306
	Port int32 `json:"port,omitempty"`
	// +kubebuilder:validation:Required
	VolumeClaimTemplate corev1.PersistentVolumeClaimSpec `json:"volumeClaimTemplate" webhook:"inmutable"`

	MyCnf                *string                      `json:"myCnf,omitempty" webhook:"inmutable"`
	MyCnfConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"myCnfConfigMapKeyRef,omitempty" webhook:"inmutableinit"`

	BootstrapFrom *RestoreSource `json:"bootstrapFrom,omitempty" webhook:"inmutable"`

	Metrics *Metrics `json:"metrics,omitempty"`

	Replication *Replication `json:"replication,omitempty"`
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	Env     []corev1.EnvVar        `json:"env,omitempty"`
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	SecurityContext    *corev1.SecurityContext    `json:"securityContext,omitempty"`

	LivenessProbe  *corev1.Probe `json:"livenessProbe,omitempty"`
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`

	Service *Service `json:"service,omitempty"`
}

// MariaDBStatus defines the observed state of MariaDB
type MariaDBStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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

func (m *MariaDB) IsBootstrapped() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeBootstrapped)
}

func (m *MariaDB) ConfigMapValue() *string {
	return m.Spec.MyCnf
}

func (m *MariaDB) ConfigMapKeyRef() *corev1.ConfigMapKeySelector {
	return m.Spec.MyCnfConfigMapKeyRef
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
