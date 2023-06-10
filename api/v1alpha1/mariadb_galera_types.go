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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultRecoveryUnhealthyThreshold = 1 * time.Minute
	defaultRecoveryUnhealthyTimeout   = 5 * time.Minute
)

type GaleraAgent struct {
	ContainerTemplate `json:",inline"`
	// +kubebuilder:default=5555
	Port int32 `json:"port,omitempty"`
	// +kubebuilder:default=10
	RecoveryRetries int `json:"recoveryRetries,omitempty"`

	RecoveryRetryWait *metav1.Duration `json:"recoveryRetryWait,omitempty"`
}

type GaleraRecovery struct {
	UnhealthyThreshold *metav1.Duration `json:"unhealthyThreshold,omitempty"`

	UnhealthyTimeout *metav1.Duration `json:"unhealthyTimeout,omitempty"`
}

func (g *GaleraRecovery) Validate() error {
	if g.UnhealthyThreshold != nil && g.UnhealthyTimeout != nil &&
		g.UnhealthyTimeout.Duration < g.UnhealthyThreshold.Duration {
		return errors.New("unhealthyTimeout must be greater than unhealthyThreshold")
	}
	return nil
}

func (g *GaleraRecovery) UnhealthyThresholdOrDefault() time.Duration {
	if g.UnhealthyThreshold != nil {
		return g.UnhealthyThreshold.Duration
	}
	return defaultRecoveryUnhealthyThreshold
}

func (g *GaleraRecovery) UnhealthyTimeoutOrDefault() time.Duration {
	if g.UnhealthyTimeout != nil {
		return g.UnhealthyTimeout.Duration
	}
	return defaultRecoveryUnhealthyTimeout
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
