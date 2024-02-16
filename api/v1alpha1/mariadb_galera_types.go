package v1alpha1

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

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

// PrimaryGalera is the Galera configuration for the primary node.
type PrimaryGalera struct {
	// PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodIndex *int `json:"podIndex,omitempty"`
	// AutomaticFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	AutomaticFailover *bool `json:"automaticFailover,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (r *PrimaryGalera) SetDefaults() {
	if r.PodIndex == nil {
		r.PodIndex = ptr.To(0)
	}
	if r.AutomaticFailover == nil {
		r.AutomaticFailover = ptr.To(true)
	}
}

// KubernetesAuth refers to the Kubernetes authentication mechanism utilized for establishing a connection from the operator to the agent.
// The agent validates the legitimacy of the service account token provided as an Authorization header by creating a TokenReview resource.
type KubernetesAuth struct {
	// Enabled is a flag to enable KubernetesAuth
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
	// AuthDelegatorRoleName is the name of the ClusterRoleBinding that is associated with the "system:auth-delegator" ClusterRole.
	// It is necessary for creating TokenReview objects in order for the agent to validate the service account token.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AuthDelegatorRoleName string `json:"authDelegatorRoleName,omitempty"`
}

// AuthDelegatorRoleNameOrDefault defines the ClusterRoleBinding name bound to system:auth-delegator.
// It falls back to the MariaDB name if AuthDelegatorRoleName is not set.
func (k *KubernetesAuth) AuthDelegatorRoleNameOrDefault(mariadb *MariaDB) string {
	if k.AuthDelegatorRoleName != "" {
		return k.AuthDelegatorRoleName
	}
	name := fmt.Sprintf("%s-%s", mariadb.Name, mariadb.Namespace)
	parts := strings.Split(string(mariadb.UID), "-")
	if len(parts) > 0 {
		name += fmt.Sprintf("-%s", parts[0])
	}
	return name
}

// GaleraAgent is a sidecar agent that co-operates with mariadb-operator.
type GaleraAgent struct {
	// ContainerTemplate defines a template to configure Container objects.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerTemplate `json:",inline"`
	// Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Image string `json:"image,omitempty"`
	// ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`.
	// +optional
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:imagePullPolicy"}
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Port where the agent will be listening for connections.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Port int32 `json:"port,omitempty"`
	// KubernetesAuth to be used by the agent container
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	KubernetesAuth *KubernetesAuth `json:"kubernetesAuth,omitempty"`
	// GracefulShutdownTimeout is the time we give to the agent container in order to gracefully terminate in-flight requests.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	GracefulShutdownTimeout *metav1.Duration `json:"gracefulShutdownTimeout,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (r *GaleraAgent) SetDefaults() {
	if r.Image == "" {
		r.Image = "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26"
	}
	if r.ImagePullPolicy == "" {
		r.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if r.Port == 0 {
		r.Port = 5555
	}
	if r.KubernetesAuth == nil {
		r.KubernetesAuth = &KubernetesAuth{
			Enabled: true,
		}
	}
	if r.GracefulShutdownTimeout == nil {
		r.GracefulShutdownTimeout = ptr.To(metav1.Duration{Duration: 1 * time.Second})
	}
}

// GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
// More info: https://galeracluster.com/library/documentation/crash-recovery.html.
type GaleraRecovery struct {
	// Enabled is a flag to enable GaleraRecovery.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`
	// ClusterHealthyTimeout represents the duration at which a Galera cluster, that consistently failed health checks,
	// is considered unhealthy, and consequently the Galera recovery process will be initiated by the operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClusterHealthyTimeout *metav1.Duration `json:"clusterHealthyTimeout,omitempty"`
	// ClusterBootstrapTimeout is the time limit for bootstrapping a cluster.
	// Once this timeout is reached, the Galera recovery state is reset and a new cluster bootstrap will be attempted.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClusterBootstrapTimeout *metav1.Duration `json:"clusterBootstrapTimeout,omitempty"`
	// PodRecoveryTimeout is the time limit for recevorying the sequence of a Pod during the cluster recovery.
	// Once this timeout is reached, the Pod is restarted.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodRecoveryTimeout *metav1.Duration `json:"podRecoveryTimeout,omitempty"`
	// PodSyncTimeout is the time limit for a Pod to join the cluster after having performed a cluster bootstrap during the cluster recovery.
	// Once this timeout is reached, the Pod is restarted.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodSyncTimeout *metav1.Duration `json:"podSyncTimeout,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (g *GaleraRecovery) SetDefaults() {
	if g.ClusterHealthyTimeout == nil {
		g.ClusterHealthyTimeout = ptr.To(metav1.Duration{Duration: 30 * time.Second})
	}
	if g.ClusterBootstrapTimeout == nil {
		g.ClusterBootstrapTimeout = ptr.To(metav1.Duration{Duration: 3 * time.Minute})
	}
	if g.PodRecoveryTimeout == nil {
		g.PodRecoveryTimeout = ptr.To(metav1.Duration{Duration: 3 * time.Minute})
	}
	if g.PodSyncTimeout == nil {
		g.PodSyncTimeout = ptr.To(metav1.Duration{Duration: 3 * time.Minute})
	}
}

// Galera allows you to enable multi-master HA via Galera in your MariaDB cluster.
type Galera struct {
	// GaleraSpec is the Galera desired state specification.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	GaleraSpec `json:",inline"`
	// Enabled is a flag to enable Galera.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (g *Galera) SetDefaults() {
	if g.SST == "" {
		g.SST = SSTMariaBackup
	}
	if g.ReplicaThreads == 0 {
		g.ReplicaThreads = 1
	}
	if reflect.ValueOf(g.InitContainer).IsZero() {
		g.InitContainer = Container{
			Image:           "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
	}
	if reflect.ValueOf(g.VolumeClaimTemplate).IsZero() {
		g.VolumeClaimTemplate = VolumeClaimTemplate{
			PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": resource.MustParse("100Mi"),
					},
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
			},
		}
	}
	g.Primary.SetDefaults()
	g.Agent.SetDefaults()

	if g.Recovery == nil {
		g.Recovery = &GaleraRecovery{
			Enabled: true,
		}
	}
	if ptr.Deref(g.Recovery, GaleraRecovery{}).Enabled {
		g.Recovery.SetDefaults()
	}
}

// GaleraSpec is the Galera desired state specification.
type GaleraSpec struct {
	// Primary is the Galera configuration for the primary node.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Primary PrimaryGalera `json:"primary,omitempty"`
	// SST is the Snapshot State Transfer used when new Pods join the cluster.
	// More info: https://galeracluster.com/library/documentation/sst.html.
	// +optional
	// +kubebuilder:validation:Enum=rsync;mariabackup;mysqldump
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SST SST `json:"sst,omitempty"`
	// ReplicaThreads is the number of replica threads used to apply Galera write sets in parallel.
	// More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_slave_threads.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicaThreads int `json:"replicaThreads,omitempty"`
	// GaleraAgent is a sidecar agent that co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Agent GaleraAgent `json:"agent,omitempty"`
	// GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
	// More info: https://galeracluster.com/library/documentation/crash-recovery.html.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Recovery *GaleraRecovery `json:"recovery,omitempty"`
	// InitContainer is an init container that co-operates with mariadb-operator.
	// More info: https://github.com/mariadb-operator/init.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	InitContainer Container `json:"initContainer,omitempty"`
	// VolumeClaimTemplate is a template for the PVC that will contain the Galera configuration files
	// shared between the InitContainer, Agent and MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	VolumeClaimTemplate VolumeClaimTemplate `json:"volumeClaimTemplate,omitempty"`
}

// GaleraRecoveryBootstrap indicates when and in which Pod the cluster bootstrap process has been performed.
type GaleraRecoveryBootstrap struct {
	Time *metav1.Time `json:"time,omitempty"`
	Pod  *string      `json:"pod,omitempty"`
}

// GaleraRecoveryStatus is the current state of the Galera recovery process.
type GaleraRecoveryStatus struct {
	// State is a per Pod representation of the Galera state file (grastate.dat).
	State map[string]*recovery.GaleraState `json:"state,omitempty"`
	// State is a per Pod representation of the sequence recovery process.
	Recovered map[string]*recovery.Bootstrap `json:"recovered,omitempty"`
	// Bootstrap indicates when and in which Pod the cluster bootstrap process has been performed.
	Bootstrap *GaleraRecoveryBootstrap `json:"bootstrap,omitempty"`
	// PodsRestarted that the Pods have been restarted after the cluster bootstrap.
	PodsRestarted *bool `json:"podsRestarted,omitempty"`
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
