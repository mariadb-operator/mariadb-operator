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

// KubernetesAuth refers to the Kubernetes authentication mechanism utilized for establishing a connection from the operator to the agent.
// The agent validates the legitimacy of the service account token provided as an Authorization header by creating a TokenReview resource.
type KubernetesAuth struct {
	// Enabled is a flag to enable KubernetesAuth
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled"`
	// AuthDelegatorRoleName is the name of the ClusterRoleBinding that is associated with the "system:auth-delegator" ClusterRole.
	// It is necessary for creating TokenReview objects in order for the agent to validate the service account token.
	// +optional
	AuthDelegatorRoleName string `json:"authDelegatorRoleName,omitempty"`
}

// AuthDelegatorRoleNameOrDefault defines the ClusterRoleBinding name bound to system:auth-delegator.
// It falls back to the MariaDB name if AuthDelegatorRoleName is not set.
func (k *KubernetesAuth) AuthDelegatorRoleNameOrDefault(mariadb *MariaDB) string {
	if k.AuthDelegatorRoleName != "" {
		return k.AuthDelegatorRoleName
	}
	return mariadb.Name
}

// GaleraAgent is a sidecar agent that co-operates with mariadb-operator.
// More info: https://github.com/mariadb-operator/agent.
type GaleraAgent struct {
	// ContainerTemplate to be used in the agent container.
	// +optional
	ContainerTemplate `json:",inline"`
	// Port to be used by the agent container
	// +kubebuilder:default=5555
	// +optional
	Port int32 `json:"port,omitempty"`
	// KubernetesAuth to be used by the agent container
	// +optional
	KubernetesAuth *KubernetesAuth `json:"kubernetesAuth,omitempty"`
	// GracefulShutdownTimeout is the time we give to the agent container in order to gracefully terminate in-flight requests.
	// +optional
	GracefulShutdownTimeout *metav1.Duration `json:"gracefulShutdownTimeout,omitempty"`
}

// GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
// More info: https://galeracluster.com/library/documentation/crash-recovery.html.
type GaleraRecovery struct {
	// Enabled is a flag to enable GaleraRecovery.
	// +optional
	Enabled bool `json:"enabled"`
	// ClusterHealthyTimeout represents the duration at which a Galera cluster, that consistently failed health checks,
	// is considered unhealthy, and consequently the Galera recovery process will be initiated by the operator.
	// +optional
	ClusterHealthyTimeout *metav1.Duration `json:"clusterHealthyTimeout,omitempty"`
	// ClusterBootstrapTimeout is the time limit for bootstrapping a cluster.
	// Once this timeout is reached, the Galera recovery state is reset and a new cluster bootstrap will be attempted.
	// +optional
	ClusterBootstrapTimeout *metav1.Duration `json:"clusterBootstrapTimeout,omitempty"`
	// PodRecoveryTimeout is the time limit for executing the recovery sequence within a Pod.
	// This process includes enabling the recovery mode in the Galera configuration file, restarting the Pod
	// and retrieving the sequence from a log file.
	// +optional
	PodRecoveryTimeout *metav1.Duration `json:"podRecoveryTimeout,omitempty"`
	// PodSyncTimeout is the time limit we give to a Pod to reach the Sync state.
	// Once this timeout is reached, the Pod is restarted.
	// +optional
	PodSyncTimeout *metav1.Duration `json:"podSyncTimeout,omitempty"`
}

// SST is the Snapshot State Transfer used when new Pods join the cluster.
// More info: https://galeracluster.com/library/documentation/sst.html.
type SST string

const (
	// SSTRsync is an SST based on rsync.
	SSTRsync SST = "rsync"
	// SSTMariaBackup is an SST based on mariabackup. It is the recommended SST.
	SSTMariaBackup SST = "mariabackup"
	// SSTMysqldump is an SST based on mysqldump.
	SSTMysqldump SST = "mysqldump"
)

// Validate returns an error if the SST is not valid.
func (s SST) Validate() error {
	switch s {
	case SSTMariaBackup, SSTRsync, SSTMysqldump:
		return nil
	default:
		return fmt.Errorf("invalid SST: %v", s)
	}
}

// MariaDBFormat formats the SST so it can be used in Galera config files.
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

// Galera allows you to enable multi-master HA via Galera in your MariaDB cluster.
type Galera struct {
	// GaleraSpec is the Galera desired state specification.
	// +optional
	GaleraSpec `json:",inline"`
	// Enabled is a flag to enable Galera.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// GaleraSpec is the Galera desired state specification.
type GaleraSpec struct {
	// SST is the Snapshot State Transfer used when new Pods join the cluster.
	// More info: https://galeracluster.com/library/documentation/sst.html.
	// +optional
	SST *SST `json:"sst,omitempty"`
	// ReplicaThreads is the number of replica threads used to apply Galera write sets in parallel.
	// More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_slave_threads.
	// +optional
	ReplicaThreads *int `json:"replicaThreads,omitempty"`
	// GaleraAgent is a sidecar agent that co-operates with mariadb-operator.
	// More info: https://github.com/mariadb-operator/agent.
	// +optional
	Agent *GaleraAgent `json:"agent,omitempty"`
	// GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
	// More info: https://galeracluster.com/library/documentation/crash-recovery.html.
	// +optional
	Recovery *GaleraRecovery `json:"recovery,omitempty"`
	// InitContainer is an init container that co-operates with mariadb-operator.
	// More info: https://github.com/mariadb-operator/init.
	// +optional
	InitContainer *ContainerTemplate `json:"initContainer,omitempty"`
	// VolumeClaimTemplate is a template for the PVC that will contain the Galera configuration files
	// shared between the InitContainer, Agent and MariaDB.
	// +optional
	VolumeClaimTemplate *corev1.PersistentVolumeClaimSpec `json:"volumeClaimTemplate,omitempty"`
}

// FillWithDefaults fills the current GaleraSpec object with DefaultGaleraSpec.
// This enables having minimal GaleraSpec objects and provides sensible defaults.
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

	// DefaultGaleraSpec provides sensible defaults for the GaleraSpec.
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
			Enabled:                 true,
			ClusterHealthyTimeout:   &oneMinute,
			ClusterBootstrapTimeout: &fiveMinutes,
			PodRecoveryTimeout:      &threeMinutes,
			PodSyncTimeout:          &threeMinutes,
		},
		InitContainer: &ContainerTemplate{
			Image: Image{
				Repository: "ghcr.io/mariadb-operator/init",
				Tag:        "v0.0.3",
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

// GaleraRecoveryBootstrap indicates when and in which Pod the cluster bootstrap process has been performed.
type GaleraRecoveryBootstrap struct {
	Time *metav1.Time `json:"time,omitempty"`
	Pod  *string      `json:"pod,omitempty"`
}

// GaleraRecoveryStatus is the current state of the Galera recovery process.
type GaleraRecoveryStatus struct {
	// State is a per Pod representation of the Galera state file (grastate.dat).
	State map[string]*agentgalera.GaleraState `json:"state,omitempty"`
	// State is a per Pod representation of the sequence recovery process.
	Recovered map[string]*agentgalera.Bootstrap `json:"recovered,omitempty"`
	// Bootstrap indicates when and in which Pod the cluster bootstrap process has been performed.
	Bootstrap *GaleraRecoveryBootstrap `json:"bootstrap,omitempty"`
}

// HasGaleraReadyCondition indicates whether the MariaDB object has a GaleraReady status condition.
// This means that the Galera cluster is healthy.
func (m *MariaDB) HasGaleraReadyCondition() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeGaleraReady)
}

// HasGaleraNotReadyCondition indicates whether the MariaDB object has a non GaleraReady status condition.
// This means that the Galera cluster is not healthy.
func (m *MariaDB) HasGaleraNotReadyCondition() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeGaleraReady)
}

// HasGaleraConfiguredCondition indicates whether the MariaDB object has a GaleraConfigured status condition.
// This means that the cluster has been successfully configured the first time.
func (m *MariaDB) HasGaleraConfiguredCondition() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeGaleraConfigured)
}
