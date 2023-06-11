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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GaleraAgent struct {
	ContainerTemplate `json:",inline"`
	// +kubebuilder:default=5555
	Port int32 `json:"port,omitempty"`
}

type GaleraRecovery struct {
	HealthyTimeout *metav1.Duration `json:"healthyTimeout,omitempty"`

	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

func (g *GaleraRecovery) HealthyTimeoutOrDefault() time.Duration {
	if g.HealthyTimeout != nil {
		return g.HealthyTimeout.Duration
	}
	return 1 * time.Minute
}

func (g *GaleraRecovery) TimeoutOrDefault() time.Duration {
	if g.Timeout != nil {
		return g.Timeout.Duration
	}
	return 1 * time.Minute
}

type Galera struct {
	// +kubebuilder:validation:Required
	Agent GaleraAgent `json:"agent"`
	// +kubebuilder:validation:Required
	Recovery GaleraRecovery `json:"recovery"`
	// +kubebuilder:default=1
	ReplicaThreads int `json:"replicaThreads,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	LivenessProbe bool `json:"livenessProbe"`
	// +kubebuilder:validation:Required
	InitContainer ContainerTemplate `json:"initContainer"`
	// +kubebuilder:validation:Required
	VolumeClaimTemplate corev1.PersistentVolumeClaimSpec `json:"volumeClaimTemplate" webhook:"inmutable"`
}

func (m *MariaDB) HasGaleraReadyCondition() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeGaleraReady)
}

func (m *MariaDB) HasGaleraNotReadyCondition() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeGaleraReady)
}

func (m *MariaDB) HasGaleraConfiguredCondition() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeGaleraConfigured)
}
