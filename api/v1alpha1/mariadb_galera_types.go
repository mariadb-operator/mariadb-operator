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
	"time"

	agentgalera "github.com/mariadb-operator/agent/pkg/galera"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GaleraAgent struct {
	ContainerTemplate `json:",inline"`
	// +kubebuilder:default=5555
	Port int32 `json:"port,omitempty"`

	GracefulShutdownTimeout *metav1.Duration `json:"gracefulShutdownTimeout,omitempty"`
}

// TODO: default using a mutating webhook
func (g *GaleraAgent) GracefulShutdownTimeoutOrDefault() time.Duration {
	if g.GracefulShutdownTimeout != nil {
		return g.GracefulShutdownTimeout.Duration
	}
	return 5 * time.Second
}

type GaleraRecovery struct {
	ClusterHealthyTimeout *metav1.Duration `json:"clusterHealthyTimeout,omitempty"`

	ClusterBootstrapTimeout *metav1.Duration `json:"clusterBootstrapTimeout,omitempty"`

	PodRecoveryTimeout *metav1.Duration `json:"podRecoveryTimeout,omitempty"`
}

// TODO: default using a mutating webhook
func (g *GaleraRecovery) ClusterHealthyTimeoutOrDefault() time.Duration {
	if g.ClusterHealthyTimeout != nil {
		return g.ClusterHealthyTimeout.Duration
	}
	return 1 * time.Minute
}

// TODO: default using a mutating webhook
func (g *GaleraRecovery) ClusterBootstrapTimeoutOrDefault() time.Duration {
	if g.ClusterBootstrapTimeout != nil {
		return g.ClusterBootstrapTimeout.Duration
	}
	return 5 * time.Minute
}

// TODO: default using a mutating webhook
func (g *GaleraRecovery) PodRecoveryTimeoutOrDefault() time.Duration {
	if g.PodRecoveryTimeout != nil {
		return g.PodRecoveryTimeout.Duration
	}
	return 1 * time.Minute
}

type SST string

const (
	SSTRsync       SST = "rsync"
	SSTMariaBackup SST = "mariabackup"
	SSTMysqldump   SST = "mysqldump"
)

func (s SST) Validate() error {
	switch s {
	case SSTMariaBackup, SSTRsync, SSTMysqldump:
		return nil
	default:
		return fmt.Errorf("invalid SST: %v", s)
	}
}

func (s SST) MariaDBFormat() (string, error) {
	switch s {
	case SSTRsync:
		return "rsync", nil
	case SSTMariaBackup:
		return "mariabackup", nil
	case SSTMysqldump:
		return "mysqldump", nil
	default:
		return "", fmt.Errorf("invalid SST: %v", s)
	}
}

type Galera struct {
	// +kubebuilder:default=mariabackup
	SST SST `json:"sst"`
	// +kubebuilder:default=1
	ReplicaThreads int `json:"replicaThreads,omitempty"`
	// +kubebuilder:validation:Required
	Agent GaleraAgent `json:"agent"`
	// +kubebuilder:validation:Required
	Recovery GaleraRecovery `json:"recovery"`
	// +kubebuilder:validation:Required
	InitContainer ContainerTemplate `json:"initContainer"`
	// +kubebuilder:validation:Required
	VolumeClaimTemplate corev1.PersistentVolumeClaimSpec `json:"volumeClaimTemplate" webhook:"inmutable"`
}

// TODO: move galera.GaleraState and galera.Bootstrap to this package ?
type GaleraRecoveryStatus struct {
	State         map[string]*agentgalera.GaleraState `json:"state,omitempty"`
	Recovered     map[string]*agentgalera.Bootstrap   `json:"recovered,omitempty"`
	BootstrapTime *metav1.Time                        `json:"bootstrapTime,omitempty"`
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
