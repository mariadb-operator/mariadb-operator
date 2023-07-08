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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubernetesAuth struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	AuthDelegatorRoleName string `json:"authDelegatorRoleName,omitempty"`
}

func (k *KubernetesAuth) AuthDelegatorRoleNameOrDefault(mariadb *MariaDB) string {
	if k.AuthDelegatorRoleName != "" {
		return k.AuthDelegatorRoleName
	}
	return mariadb.Name
}

type GaleraAgent struct {
	ContainerTemplate `json:",inline"`
	// +kubebuilder:default=5555
	Port int32 `json:"port,omitempty"`

	KubernetesAuth *KubernetesAuth `json:"kubernetesAuth,omitempty"`

	GracefulShutdownTimeout *metav1.Duration `json:"gracefulShutdownTimeout,omitempty"`
}

type GaleraRecovery struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	ClusterHealthyTimeout *metav1.Duration `json:"clusterHealthyTimeout,omitempty"`

	ClusterBootstrapTimeout *metav1.Duration `json:"clusterBootstrapTimeout,omitempty"`

	PodRecoveryTimeout *metav1.Duration `json:"podRecoveryTimeout,omitempty"`

	PodSyncTimeout *metav1.Duration `json:"podSyncTimeout,omitempty"`
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
	GaleraSpec `json:",inline"`

	Enabled bool `json:"enabled,omitempty"`
}

type GaleraSpec struct {
	SST *SST `json:"sst,omitempty"`

	ReplicaThreads *int `json:"replicaThreads,omitempty"`

	Agent *GaleraAgent `json:"agent,omitempty"`

	Recovery *GaleraRecovery `json:"recovery,omitempty"`

	InitContainer *ContainerTemplate `json:"initContainer,omitempty"`

	VolumeClaimTemplate *corev1.PersistentVolumeClaimSpec `json:"volumeClaimTemplate,omitempty"`
}

func (g *GaleraSpec) FillWithDefaults() {
	if g.SST == nil {
		sst := *DefaultGaleraSpec.SST
		g.SST = &sst
	}
	if g.ReplicaThreads == nil {
		replicaThreads := *DefaultGaleraSpec.ReplicaThreads
		g.ReplicaThreads = &replicaThreads
	}
	if g.Agent == nil {
		agent := *DefaultGaleraSpec.Agent
		g.Agent = &agent
	}
	if g.Recovery == nil {
		recovery := *DefaultGaleraSpec.Recovery
		g.Recovery = &recovery
	}
	if g.InitContainer == nil {
		initContainer := *DefaultGaleraSpec.InitContainer
		g.InitContainer = &initContainer
	}
	if g.VolumeClaimTemplate == nil {
		volumeClaimTemplate := *DefaultGaleraSpec.VolumeClaimTemplate
		g.VolumeClaimTemplate = &volumeClaimTemplate
	}
}

var (
	fiveSeconds      = metav1.Duration{Duration: 5 * time.Second}
	oneMinute        = metav1.Duration{Duration: 1 * time.Minute}
	fiveMinutes      = metav1.Duration{Duration: 5 * time.Minute}
	threeMinutes     = metav1.Duration{Duration: 3 * time.Minute}
	sst              = SSTMariaBackup
	replicaThreads   = 1
	storageClassName = "default"

	DefaultGaleraSpec = GaleraSpec{
		SST:            &sst,
		ReplicaThreads: &replicaThreads,
		Agent: &GaleraAgent{
			ContainerTemplate: ContainerTemplate{
				Image: Image{
					Repository: "ghcr.io/mariadb-operator/agent",
					Tag:        "v0.0.2",
					PullPolicy: corev1.PullIfNotPresent,
				},
			},
			Port: 5555,
			KubernetesAuth: &KubernetesAuth{
				Enabled: true,
			},
			GracefulShutdownTimeout: &fiveSeconds,
		},
		Recovery: &GaleraRecovery{
			ClusterHealthyTimeout:   &oneMinute,
			ClusterBootstrapTimeout: &fiveMinutes,
			PodRecoveryTimeout:      &threeMinutes,
			PodSyncTimeout:          &threeMinutes,
		},
		InitContainer: &ContainerTemplate{
			Image: Image{
				Repository: "ghcr.io/mariadb-operator/init",
				Tag:        "v0.0.2",
				PullPolicy: corev1.PullIfNotPresent,
			},
		},
		VolumeClaimTemplate: &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("50Mi"),
				},
			},
			StorageClassName: &storageClassName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
		},
	}
)

type GaleraRecoveryBootstrap struct {
	Time *metav1.Time `json:"time,omitempty"`
	Pod  *string      `json:"pod,omitempty"`
}

type GaleraRecoveryStatus struct {
	State     map[string]*agentgalera.GaleraState `json:"state,omitempty"`
	Recovered map[string]*agentgalera.Bootstrap   `json:"recovered,omitempty"`
	Bootstrap *GaleraRecoveryBootstrap            `json:"bootstrap,omitempty"`
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
