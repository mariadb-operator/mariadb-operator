package v1alpha1

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/docker"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

// KubernetesAuth refers to the basic authentication mechanism utilized for establishing a connection from the operator to the agent.
type BasicAuth struct {
	// Enabled is a flag to enable BasicAuth
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
	// Username to be used for basic authentication
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Username string `json:"username,omitempty"`
	// PasswordSecretKeyRef to be used for basic authentication
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PasswordSecretKeyRef GeneratedSecretKeyRef `json:"passwordSecretKeyRef,omitempty"`
}

// SetDefaults set reasonable defaults
func (b *BasicAuth) SetDefaults(mariadb *MariaDB) {
	if !b.Enabled {
		return
	}
	if b.Username == "" {
		b.Username = "mariadb-operator"
	}
	if reflect.ValueOf(b.PasswordSecretKeyRef).IsZero() {
		b.PasswordSecretKeyRef = mariadb.AgentAuthSecretKeyRef()
	}
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
	// BasicAuth to be used by the agent container
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`
	// GracefulShutdownTimeout is the time we give to the agent container in order to gracefully terminate in-flight requests.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	GracefulShutdownTimeout *metav1.Duration `json:"gracefulShutdownTimeout,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (r *GaleraAgent) SetDefaults(mariadb *MariaDB, env *environment.OperatorEnv) error {
	if r.Image == "" {
		r.Image = env.MariadbOperatorImage
	}
	if r.Port == 0 {
		r.Port = 5555
	}

	currentNamespaceOnly, err := env.CurrentNamespaceOnly()
	if err != nil {
		return fmt.Errorf("error checking operator watch scope: %v", err)
	}
	if currentNamespaceOnly {
		if r.BasicAuth == nil {
			r.BasicAuth = &BasicAuth{
				Enabled: true,
			}
		}
	} else if r.KubernetesAuth == nil && r.BasicAuth == nil {
		r.KubernetesAuth = &KubernetesAuth{
			Enabled: true,
		}
	} else if r.BasicAuth == nil {
		r.BasicAuth = &BasicAuth{
			Enabled: true,
		}
	}
	if r.BasicAuth != nil {
		r.BasicAuth.SetDefaults(mariadb)
	}

	if r.GracefulShutdownTimeout == nil {
		r.GracefulShutdownTimeout = ptr.To(metav1.Duration{Duration: 1 * time.Second})
	}
	return nil
}

// Validate determines if a Galera Agent object is valid.
func (r *GaleraAgent) Validate() error {
	kubernetesAuth := ptr.Deref(r.KubernetesAuth, KubernetesAuth{})
	basicAuth := ptr.Deref(r.BasicAuth, BasicAuth{})
	if kubernetesAuth.Enabled && basicAuth.Enabled {
		return errors.New("Only one authentication method must be enabled: kubernetes or basic auth")
	}
	return nil
}

// GaleraRecoveryJob defines a Job used to be used to recover the Galera cluster.
type GaleraRecoveryJob struct {
	// Metadata defines additional metadata for the Galera recovery Jobs.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Metadata *Metadata `json:"metadata,omitempty"`
	// Resouces describes the compute resource requirements.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// PodAffinity indicates whether the recovery Jobs should run in the same Node as the MariaDB Pods. It defaults to true.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	PodAffinity *bool `json:"podAffinity,omitempty"`
}

// GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
// More info: https://galeracluster.com/library/documentation/crash-recovery.html.
type GaleraRecovery struct {
	// Enabled is a flag to enable GaleraRecovery.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`
	// MinClusterSize is the minimum number of replicas to consider the cluster healthy. It can be either a number of replicas (1) or a percentage (50%).
	// If Galera consistently reports less replicas than this value for the given 'ClusterHealthyTimeout' interval, a cluster recovery is iniated.
	// It defaults to '1' replica.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MinClusterSize *intstr.IntOrString `json:"minClusterSize,omitempty"`
	// ClusterMonitorInterval represents the interval used to monitor the Galera cluster health.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClusterMonitorInterval *metav1.Duration `json:"clusterMonitorInterval,omitempty"`
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
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodRecoveryTimeout *metav1.Duration `json:"podRecoveryTimeout,omitempty"`
	// PodSyncTimeout is the time limit for a Pod to join the cluster after having performed a cluster bootstrap during the cluster recovery.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodSyncTimeout *metav1.Duration `json:"podSyncTimeout,omitempty"`
	// ForceClusterBootstrapInPod allows you to manually initiate the bootstrap process in a specific Pod.
	// IMPORTANT: Use this option only in exceptional circumstances. Not selecting the Pod with the highest sequence number may result in data loss.
	// IMPORTANT: Ensure you unset this field after completing the bootstrap to allow the operator to choose the appropriate Pod to bootstrap from in an event of cluster recovery.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ForceClusterBootstrapInPod *string `json:"forceClusterBootstrapInPod,omitempty"`
	// Job defines a Job that co-operates with mariadb-operator by performing the Galera cluster recovery .
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Job *GaleraRecoveryJob `json:"job,omitempty"`
}

// Validate determines whether a GaleraRecovery is valid.
func (g *GaleraRecovery) Validate(mdb *MariaDB) error {
	if !g.Enabled {
		return nil
	}
	if g.MinClusterSize != nil {
		_, err := intstr.GetScaledValueFromIntOrPercent(g.MinClusterSize, 0, false)
		if err != nil {
			return err
		}
		if g.MinClusterSize.Type == intstr.Int {
			minClusterSize := g.MinClusterSize.IntValue()
			if minClusterSize < 0 || minClusterSize > int(mdb.Spec.Replicas) {
				return fmt.Errorf("'spec.galera.recovery.minClusterSize' out of 'spec.replicas' bounds: %d", minClusterSize)
			}
		}
	}
	if g.ForceClusterBootstrapInPod != nil {
		if err := statefulset.ValidPodName(mdb.ObjectMeta, int(mdb.Spec.Replicas), *g.ForceClusterBootstrapInPod); err != nil {
			return fmt.Errorf("'spec.galera.recovery.forceClusterBootstrapInPod' invalid: %v", err)
		}
	}
	return nil
}

// SetDefaults sets reasonable defaults.
func (g *GaleraRecovery) SetDefaults(mdb *MariaDB) {
	if g.MinClusterSize == nil {
		g.MinClusterSize = ptr.To(intstr.FromInt(1))
	}
	if g.ClusterMonitorInterval == nil {
		g.ClusterMonitorInterval = ptr.To(metav1.Duration{Duration: 10 * time.Second})
	}
	if g.ClusterHealthyTimeout == nil {
		g.ClusterHealthyTimeout = ptr.To(metav1.Duration{Duration: 30 * time.Second})
	}
	if g.ClusterBootstrapTimeout == nil {
		g.ClusterBootstrapTimeout = ptr.To(metav1.Duration{Duration: 10 * time.Minute})
	}
	if g.PodRecoveryTimeout == nil {
		g.PodRecoveryTimeout = ptr.To(metav1.Duration{Duration: 5 * time.Minute})
	}
	if g.PodSyncTimeout == nil {
		g.PodSyncTimeout = ptr.To(metav1.Duration{Duration: 5 * time.Minute})
	}
}

// HasMinClusterSize returns whether the current cluster has the minimum number of replicas. If not, a cluster recovery will be performed.
func (g *GaleraRecovery) HasMinClusterSize(currentSize int, mdb *MariaDB) (bool, error) {
	minClusterSize := ptr.Deref(g.MinClusterSize, intstr.FromInt(1))
	scaled, err := intstr.GetScaledValueFromIntOrPercent(&minClusterSize, int(mdb.Spec.Replicas), true)
	if err != nil {
		return false, err
	}
	return currentSize >= scaled, nil
}

// GaleraConfig defines storage options for the Galera configuration files.
type GaleraConfig struct {
	// ReuseStorageVolume indicates that storage volume used by MariaDB should be reused to store the Galera configuration files.
	// It defaults to false, which implies that a dedicated volume for the Galera configuration files is provisioned.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	ReuseStorageVolume *bool `json:"reuseStorageVolume,omitempty" webhook:"inmutableinit"`
	// VolumeClaimTemplate is a template for the PVC that will contain the Galera configuration files shared between the InitContainer, Agent and MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	VolumeClaimTemplate *VolumeClaimTemplate `json:"volumeClaimTemplate,omitempty" webhook:"inmutableinit"`
}

// SetDefaults sets reasonable defaults.
func (g *GaleraConfig) SetDefaults() {
	if g.ReuseStorageVolume == nil {
		g.ReuseStorageVolume = ptr.To(false)
	}
	if !ptr.Deref(g.ReuseStorageVolume, false) && g.VolumeClaimTemplate == nil {
		g.VolumeClaimTemplate = &VolumeClaimTemplate{
			PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
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
}

// Galera allows you to enable multi-master HA via Galera in your MariaDB cluster.
type Galera struct {
	// GaleraSpec is the Galera desired state specification.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	GaleraSpec `json:",inline"`
	// Enabled is a flag to enable Galera.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (g *Galera) SetDefaults(mdb *MariaDB, env *environment.OperatorEnv) error {
	if g.SST == "" {
		g.SST = SSTMariaBackup
	}
	if g.AvailableWhenDonor == nil {
		g.AvailableWhenDonor = ptr.To(false)
	}
	if g.GaleraLibPath == "" {
		g.GaleraLibPath = env.MariadbGaleraLibPath
	}
	if g.ReplicaThreads == 0 {
		g.ReplicaThreads = 1
	}

	if reflect.ValueOf(g.InitContainer).IsZero() {
		g.InitContainer = Container{
			Image: env.MariadbOperatorImage,
		}
	}
	g.Primary.SetDefaults()
	if err := g.Agent.SetDefaults(mdb, env); err != nil {
		return fmt.Errorf("error setting agent defaults: %v", err)
	}
	g.Config.SetDefaults()

	if g.Recovery == nil {
		g.Recovery = &GaleraRecovery{
			Enabled: true,
		}
	}
	if ptr.Deref(g.Recovery, GaleraRecovery{}).Enabled {
		g.Recovery.SetDefaults(mdb)
	}

	if g.InitJob != nil {
		g.InitJob.SetDefaults(mdb.ObjectMeta)
	}

	autoUpdateDataPlane := ptr.Deref(mdb.Spec.UpdateStrategy.AutoUpdateDataPlane, false)
	if autoUpdateDataPlane {
		initBumped, err := docker.SetTagOrDigest(env.MariadbOperatorImage, g.InitContainer.Image)
		if err != nil {
			return fmt.Errorf("error bumping Galera init image: %v", err)
		}
		g.InitContainer.Image = initBumped

		agentBumped, err := docker.SetTagOrDigest(env.MariadbOperatorImage, g.Agent.Image)
		if err != nil {
			return fmt.Errorf("error bumping Galera agent image: %v", err)
		}
		g.Agent.Image = agentBumped
	}

	return nil
}

// GaleraSpec is the Galera desired state specification.
type GaleraSpec struct {
	// Primary is the Galera configuration for the primary node.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Primary PrimaryGalera `json:"primary,omitempty"`
	// SST is the Snapshot State Transfer used when new Pods join the cluster.
	// More info: https://galeracluster.com/library/documentation/sst.html.
	// +optional
	// +kubebuilder:validation:Enum=rsync;mariabackup;mysqldump
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SST SST `json:"sst,omitempty"`
	// AvailableWhenDonor indicates whether a donor node should be responding to queries. It defaults to false.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	AvailableWhenDonor *bool `json:"availableWhenDonor,omitempty"`
	// GaleraLibPath is a path inside the MariaDB image to the wsrep provider plugin. It is defaulted if not provided.
	// More info: https://galeracluster.com/library/documentation/mysql-wsrep-options.html#wsrep-provider.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	GaleraLibPath string `json:"galeraLibPath,omitempty"`
	// ReplicaThreads is the number of replica threads used to apply Galera write sets in parallel.
	// More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_slave_threads.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ReplicaThreads int `json:"replicaThreads,omitempty"`
	// ProviderOptions is map of Galera configuration parameters.
	// More info: https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_provider_options.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ProviderOptions map[string]string `json:"providerOptions,omitempty"`
	// GaleraAgent is a sidecar agent that co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Agent GaleraAgent `json:"agent,omitempty"`
	// GaleraRecovery is the recovery process performed by the operator whenever the Galera cluster is not healthy.
	// More info: https://galeracluster.com/library/documentation/crash-recovery.html.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Recovery *GaleraRecovery `json:"recovery,omitempty"`
	// InitContainer is an init container that runs in the MariaDB Pod and co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InitContainer Container `json:"initContainer,omitempty"`
	// InitJob defines a Job that co-operates with mariadb-operator by performing initialization tasks.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InitJob *Job `json:"initJob,omitempty"`
	// GaleraConfig defines storage options for the Galera configuration files.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Config GaleraConfig `json:"config,omitempty"`
}

// GaleraBootstrapStatus indicates when and in which Pod the cluster bootstrap process has been performed.
type GaleraBootstrapStatus struct {
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
	Bootstrap *GaleraBootstrapStatus `json:"bootstrap,omitempty"`
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

// IsGaleraInitialized indicates that the Galera init Job has successfully completed.
func (m *MariaDB) IsGaleraInitialized() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeGaleraInitialized)
}

// IsGaleraInitializing indicates that the Galera init Job is running.
func (m *MariaDB) IsGaleraInitializing() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeGaleraInitialized)
}
