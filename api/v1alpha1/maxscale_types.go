package v1alpha1

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// MaxScaleServer defines a MariaDB server to forward traffic to.
type MaxScaleServer struct {
	// Name is the identifier of the MariaDB server.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Address is the network address of the MariaDB server.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Address string `json:"address"`
	// Port is the network port of the MariaDB server. If not provided, it defaults to 3306.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	Port int32 `json:"port,omitempty"`
	// Protocol is the MaxScale protocol to use when communicating with this MariaDB server. If not provided, it defaults to MariaDBBackend.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Protocol string `json:"protocol,omitempty"`
	// Maintenance indicates whether the server is in maintenance mode.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Maintenance bool `json:"maintenance,omitempty"`
	// Params defines extra parameters to pass to the server.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Params map[string]string `json:"params,omitempty"`
}

// SetDefaults sets default values.
func (m *MaxScaleServer) SetDefaults() {
	if m.Port == 0 {
		m.Port = 3306
	}
	if m.Protocol == "" {
		m.Protocol = "MariaDBBackend"
	}
}

// SuspendTemplate indicates whether the current resource should be suspended or not. Feature flag --feature-maxscale-suspend is required in the controller to enable this.
type SuspendTemplate struct {
	// Suspend indicates whether the current resource should be suspended or not. Feature flag --feature-maxscale-suspend is required in the controller to enable this.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Suspend bool `json:"suspend,omitempty"`
}

// MonitorModule defines the type of monitor module
type MonitorModule string

const (
	// MonitorModuleMariadb is a monitor to be used with MariaDB servers.
	MonitorModuleMariadb MonitorModule = "mariadbmon"
	// MonitorModuleGalera is a monitor to be used with Galera servers.
	MonitorModuleGalera MonitorModule = "galeramon"
)

// CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors.
// See: https://mariadb.com/docs/server/architecture/components/maxscale/monitors/mariadbmon/use-cooperative-locking-ha-maxscale-mariadb-monitor/
type CooperativeMonitoring string

const (
	// CooperativeMonitoringMajorityOfAll requires a lock from the majority of the MariaDB servers, even the ones that are down.
	CooperativeMonitoringMajorityOfAll CooperativeMonitoring = "majority_of_all"
	// CooperativeMonitoringMajorityOfRunning requires a lock from the majority of the MariaDB servers.
	CooperativeMonitoringMajorityOfRunning CooperativeMonitoring = "majority_of_running"
)

// MaxScaleMonitor monitors MariaDB server instances
type MaxScaleMonitor struct {
	SuspendTemplate `json:",inline"`
	// Name is the identifier of the monitor. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Module is the module to use to monitor MariaDB servers.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=mariadbmon;galeramon
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Module MonitorModule `json:"module" webhook:"inmutable"`
	// Interval used to monitor MariaDB servers. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Interval metav1.Duration `json:"interval,omitempty"`
	// CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors. It is defaulted when HA is enabled.
	// +optional
	// +kubebuilder:validation:Enum=majority_of_all;majority_of_running
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	CooperativeMonitoring *CooperativeMonitoring `json:"cooperativeMonitoring,omitempty"`
	// Params defines extra parameters to pass to the monitor.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Params map[string]string `json:"params,omitempty"`
}

// SetCondition sets a status condition to MaxScale
func (m *MaxScaleMonitor) SetDefaults(mxs *MaxScale) {
	if m.Name == "" {
		m.Name = fmt.Sprintf("%s-monitor", string(m.Module))
	}
	if m.Interval == (metav1.Duration{}) {
		m.Interval = metav1.Duration{Duration: 2 * time.Second}
	}
	if mxs.IsHAEnabled() && m.CooperativeMonitoring == nil {
		m.CooperativeMonitoring = ptr.To(CooperativeMonitoringMajorityOfAll)
	}
}

// MaxScaleListener defines how the MaxScale server will listen for connections.
type MaxScaleListener struct {
	SuspendTemplate `json:",inline"`
	// Name is the identifier of the listener. It is defaulted if not provided
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Port is the network port where the MaxScale server will listen.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	Port int32 `json:"port,omitempty"`
	// Protocol is the MaxScale protocol to use when communicating with the client. If not provided, it defaults to MariaDBProtocol.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Protocol string `json:"protocol,omitempty"`
	// Params defines extra parameters to pass to the listener.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Params map[string]string `json:"params,omitempty"`
}

// SetDefaults sets default values.
func (m *MaxScaleListener) SetDefaults(svc *MaxScaleService) {
	if m.Name == "" {
		m.Name = fmt.Sprintf("%s-listener", svc.Name)
	}
	if m.Protocol == "" {
		m.Protocol = "MariaDBProtocol"
	}
}

// ServiceRouter defines the type of service router.
type ServiceRouter string

const (
	// ServiceRouterReadWriteSplit splits the load based on the queries. Write queries are performed on master and read queries on the replicas.
	ServiceRouterReadWriteSplit ServiceRouter = "readwritesplit"
	// ServiceRouterReadConnRoute splits the load based on the connections. Each connection is assigned to a server.
	ServiceRouterReadConnRoute ServiceRouter = "readconnroute"
)

// Services define how the traffic is forwarded to the MariaDB servers.
type MaxScaleService struct {
	SuspendTemplate `json:",inline"`
	// Name is the identifier of the MaxScale service.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Router is the type of router to use.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=readwritesplit;readconnroute
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Router ServiceRouter `json:"router" webhook:"inmutable"`
	// MaxScaleListener defines how the MaxScale server will listen for connections.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Listener MaxScaleListener `json:"listener" webhook:"inmutable"`
	// Params defines extra parameters to pass to the monitor.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Params map[string]string `json:"params,omitempty"`
}

// SetDefaults sets default values.
func (m *MaxScaleService) SetDefaults() {
	m.Listener.SetDefaults(m)
}

// MaxScaleAdmin configures the admin REST API and GUI.
type MaxScaleAdmin struct {
	// Port where the admin REST API and GUI will be exposed.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	Port int32 `json:"port"`
	// GuiEnabled indicates whether the admin GUI should be enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	GuiEnabled *bool `json:"guiEnabled,omitempty"`
}

// SetDefaults sets default values.
func (m *MaxScaleAdmin) SetDefaults(mxs *MaxScale) {
	if m.Port == 0 {
		m.Port = 8989
	}
	if m.GuiEnabled == nil {
		m.GuiEnabled = ptr.To(true)
	}
}

// MaxScaleConfigSync defines how the config changes are replicated across replicas.
type MaxScaleConfigSync struct {
	// Database is the MariaDB logical database where the 'maxscale_config' table will be created in order to persist and synchronize config changes. If not provided, it defaults to 'mysql'.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Database string `json:"database,omitempty"`
	// Interval defines the config synchronization interval. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Interval metav1.Duration `json:"interval,omitempty"`
	// Interval defines the config synchronization timeout. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Timeout metav1.Duration `json:"timeout,omitempty"`
}

// MaxScaleConfig defines the MaxScale configuration.
type MaxScaleConfig struct {
	// Params is a key value pair of parameters to be used in the MaxScale static configuration file.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Params map[string]string `json:"params,omitempty"`
	// VolumeClaimTemplate provides a template to define the PVCs for storing MaxScale runtime configuration files. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	VolumeClaimTemplate VolumeClaimTemplate `json:"volumeClaimTemplate"`
	// Sync defines how to replicate configuration across MaxScale replicas. It is defaulted when HA is enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Sync *MaxScaleConfigSync `json:"sync,omitempty"`
}

func (m *MaxScaleConfig) SetDefaults(mxs *MaxScale) {
	if reflect.ValueOf(m.VolumeClaimTemplate).IsZero() {
		m.VolumeClaimTemplate = VolumeClaimTemplate{
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
	if mxs.IsHAEnabled() {
		if m.Sync == nil {
			m.Sync = &MaxScaleConfigSync{}
		}
		if m.Sync.Database == "" {
			m.Sync.Database = "mysql"
		}
		if m.Sync.Interval == (metav1.Duration{}) {
			m.Sync.Interval = metav1.Duration{Duration: 5 * time.Second}
		}
		if m.Sync.Timeout == (metav1.Duration{}) {
			m.Sync.Timeout = metav1.Duration{Duration: 10 * time.Second}
		}
	}
}

// MaxScaleAuth defines the credentials required for MaxScale to connect to MariaDB.
type MaxScaleAuth struct {
	// AdminUsername is an admin username to call the admin REST API. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AdminUsername string `json:"adminUsername,omitempty"`
	// AdminPasswordSecretKeyRef is Secret key reference to the admin password to call the admib REST API. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AdminPasswordSecretKeyRef corev1.SecretKeySelector `json:"adminPasswordSecretKeyRef,omitempty"`
	// DeleteDefaultAdmin determines whether the default admin user should be deleted after the initial configuration. If not provided, it defaults to true.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	DeleteDefaultAdmin *bool `json:"deleteDefaultAdmin,omitempty"`
	// ClientUsername is the user to connect to MaxScale. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClientUsername string `json:"clientUsername,omitempty"`
	// ClientPasswordSecretKeyRef is Secret key reference to the password to connect to MaxScale. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ClientPasswordSecretKeyRef corev1.SecretKeySelector `json:"clientPasswordSecretKeyRef,omitempty"`
	// ServerUsername is the user used by MaxScale to connect to MariaDB server. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ServerUsername string `json:"serverUsername,omitempty"`
	// ServerPasswordSecretKeyRef is Secret key reference to the password used by MaxScale to connect to MariaDB server. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ServerPasswordSecretKeyRef corev1.SecretKeySelector `json:"serverPasswordSecretKeyRef,omitempty"`
	// MonitorUsername is the user used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MonitorUsername string `json:"monitorUsername,omitempty"`
	// MonitorPasswordSecretKeyRef is Secret key reference to the password used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MonitorPasswordSecretKeyRef corev1.SecretKeySelector `json:"monitorPasswordSecretKeyRef,omitempty"`
	// MonitoSyncUsernamerUsername is the user used by MaxScale config sync to connect to MariaDB server. It is defaulted when HA is enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SyncUsername string `json:"syncUsername,omitempty"`
	// SyncPasswordSecretKeyRef is Secret key reference to the password used by MaxScale config to connect to MariaDB server. It is defaulted when HA is enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SyncPasswordSecretKeyRef corev1.SecretKeySelector `json:"syncPasswordSecretKeyRef,omitempty"`
}

// SetDefaults sets default values.
func (m *MaxScaleAuth) SetDefaults(mxs *MaxScale) {
	if m.AdminUsername == "" {
		m.AdminUsername = "mariadb-operator"
	}
	if m.AdminPasswordSecretKeyRef == (corev1.SecretKeySelector{}) {
		m.AdminPasswordSecretKeyRef = mxs.AdminPasswordSecretKeyRef()
	}
	if m.DeleteDefaultAdmin == nil {
		m.DeleteDefaultAdmin = ptr.To(true)
	}
	if m.ClientUsername == "" {
		m.ClientUsername = mxs.AuthClientUserKey().Name
	}
	if m.ClientPasswordSecretKeyRef == (corev1.SecretKeySelector{}) {
		m.ClientPasswordSecretKeyRef = mxs.AuthClientPasswordSecretKeyRef()
	}
	if m.ServerUsername == "" {
		m.ServerUsername = mxs.AuthServerUserKey().Name
	}
	if m.ServerPasswordSecretKeyRef == (corev1.SecretKeySelector{}) {
		m.ServerPasswordSecretKeyRef = mxs.AuthServerPasswordSecretKeyRef()
	}
	if m.MonitorUsername == "" {
		m.MonitorUsername = mxs.AuthMonitorUserKey().Name
	}
	if m.MonitorPasswordSecretKeyRef == (corev1.SecretKeySelector{}) {
		m.MonitorPasswordSecretKeyRef = mxs.AuthMonitorPasswordSecretKeyRef()
	}
	if mxs.IsHAEnabled() {
		if m.SyncUsername == "" {
			m.SyncUsername = mxs.AuthSyncUserKey().Name
		}
		if m.SyncPasswordSecretKeyRef == (corev1.SecretKeySelector{}) {
			m.SyncPasswordSecretKeyRef = mxs.AuthSyncPasswordSecretKeyRef()
		}
	}
}

// ShouldDeleteDefaultAdmin indicates whether the default admin should be deleted after initial setup.
func (m *MaxScaleAuth) ShouldDeleteDefaultAdmin() bool {
	return m.DeleteDefaultAdmin != nil && *m.DeleteDefaultAdmin
}

// MaxScaleSpec defines the desired state of MaxScale.
type MaxScaleSpec struct {
	// ContainerTemplate defines templates to configure Container objects.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerTemplate `json:",inline"`
	// PodTemplate defines templates to configure Pod objects.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodTemplate `json:",inline"`
	// Image name to be used by the MaxScale instances. The supported format is `<image>:<tag>`.
	// Only MaxScale official images are supported.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Image string `json:"image,omitempty"`
	// ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`.
	// +optional
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:imagePullPolicy","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// ImagePullSecrets is the list of pull Secrets to be used to pull the image.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" webhook:"inmutable"`
	// Servers are the MariaDB servers to forward traffic to.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Servers []MaxScaleServer `json:"servers"`
	// Services define how the traffic is forwarded to the MariaDB servers.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Services []MaxScaleService `json:"services"`
	// Monitor monitors MariaDB server instances.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Monitor MaxScaleMonitor `json:"monitor"`
	// Admin configures the admin REST API and GUI.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Admin MaxScaleAdmin `json:"admin,omitempty" webhook:"inmutable"`
	// Config defines the MaxScale configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Config MaxScaleConfig `json:"config,omitempty" webhook:"inmutable"`
	// Auth defines the credentials required for MaxScale to connect to MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Auth MaxScaleAuth `json:"auth,omitempty" webhook:"inmutable"`
	// Replicas indicates the number of desired instances.
	// +kubebuilder:default=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// PodDisruptionBudget defines the budget for replica availability.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodDisruptionBudget *PodDisruptionBudget `json:"podDisruptionBudget,omitempty"`
	// UpdateStrategy defines the update strategy for the StatefulSet object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:updateStrategy"}
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	// Service defines templates to configure the Kubernetes Service object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	KubernetesService *ServiceTemplate `json:"kubernetesService,omitempty"`
	// RequeueInterval is used to perform requeue reconcilizations. If not defined, it defaults to 10s.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	RequeueInterval *metav1.Duration `json:"requeueInterval,omitempty"`
}

// MaxScaleStatus defines the observed state of MaxScale
type MaxScaleStatus struct {
	// Conditions for the MaxScale object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Replicas indicates the number of current instances.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// PrimaryServer is the primary server reported by Maxscale.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes:Pod"}
	PrimaryServer *string `json:"primaryServer,omitempty"`
}

// SetCondition sets a status condition to MaxScale
func (s *MaxScaleStatus) SetCondition(condition metav1.Condition) {
	if s.Conditions == nil {
		s.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&s.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=mxs
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Primary Server",type="string",JSONPath=".status.primaryServer"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{MaxScale,v1alpha1},{User,v1alpha1},{Grant,v1alpha1},{Event,v1},{Service,v1},{Secret,v1},{ServiceAccount,v1},{StatefulSet,v1},{PodDisruptionBudget,v1}}

// MaxScale is the Schema for the maxscales API. It is used to define MaxScale clusters.
type MaxScale struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MaxScaleSpec   `json:"spec,omitempty"`
	Status MaxScaleStatus `json:"status,omitempty"`
}

// SetDefaults sets default values.
func (m *MaxScale) SetDefaults(env *environment.Environment) {
	if m.Spec.Image == "" {
		m.Spec.Image = env.RelatedMaxscaleImage
	}
	if m.Spec.RequeueInterval == nil {
		m.Spec.RequeueInterval = &metav1.Duration{Duration: 10 * time.Second}
	}
	for i := range m.Spec.Servers {
		m.Spec.Servers[i].SetDefaults()
	}
	for i := range m.Spec.Services {
		m.Spec.Services[i].SetDefaults()
	}
	m.Spec.Monitor.SetDefaults(m)
	m.Spec.Admin.SetDefaults(m)
	m.Spec.Config.SetDefaults(m)
	m.Spec.Auth.SetDefaults(m)
}

// IsReady indicates whether the Maxscale instance is ready.
func (m *MaxScale) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReady)
}

// IsHAEnabled indicated whether high availability is enabled.
func (m *MaxScale) IsHAEnabled() bool {
	return m.Spec.Replicas > 1
}

// APIUrl returns the URL of the admin API pointing to the Kubernetes Service.
func (m *MaxScale) APIUrl() string {
	fqdn := statefulset.ServiceFQDNWithService(m.ObjectMeta, m.Name)
	return m.apiUrlWithAddress(fqdn)
}

// PodAPIUrl returns the URL of the admin API pointing to a Pod.
func (m *MaxScale) PodAPIUrl(podIndex int) string {
	fqdn := statefulset.PodFQDNWithService(m.ObjectMeta, podIndex, m.InternalServiceKey().Name)
	return m.apiUrlWithAddress(fqdn)
}

// ServerIDs returns the servers indexed by ID.
func (m *MaxScale) ServerIndex() ds.Index[MaxScaleServer] {
	return ds.NewIndex[MaxScaleServer](m.Spec.Servers, func(mss MaxScaleServer) string {
		return mss.Name
	})
}

// ServerIDs returns the IDs of the servers.
func (m *MaxScale) ServerIDs() []string {
	return ds.Keys[MaxScaleServer](m.ServerIndex())
}

// ServiceIndex returns the services indexed by ID.
func (m *MaxScale) ServiceIndex() ds.Index[MaxScaleService] {
	return ds.NewIndex[MaxScaleService](m.Spec.Services, func(mss MaxScaleService) string {
		return mss.Name
	})
}

// ServiceIDs returns the IDs of the services.
func (m *MaxScale) ServiceIDs() []string {
	return ds.Keys[MaxScaleService](m.ServiceIndex())
}

// ServiceForListener finds the service for a given listener
func (m *MaxScale) ServiceForListener(listener string) (string, error) {
	for _, svc := range m.Spec.Services {
		if svc.Listener.Name == listener {
			return svc.Name, nil
		}
	}
	return "", errors.New("service not found")
}

// Listeners returns the listeners
func (m *MaxScale) Listeners() []MaxScaleListener {
	listeners := make([]MaxScaleListener, len(m.Spec.Services))
	for i, svc := range m.Spec.Services {
		listeners[i] = svc.Listener
	}
	return listeners
}

// ListenerIndex returns the listeners indexed by ID.
func (m *MaxScale) ListenerIndex() ds.Index[MaxScaleListener] {
	return ds.NewIndex[MaxScaleListener](m.Listeners(), func(mss MaxScaleListener) string {
		return mss.Name
	})
}

// ListenerIDs returns the IDs of the listeners.
func (m *MaxScale) ListenerIDs() []string {
	return ds.Keys[MaxScaleListener](m.ListenerIndex())
}

func (m *MaxScale) apiUrlWithAddress(addr string) string {
	return fmt.Sprintf("http://%s:%d", addr, m.Spec.Admin.Port)
}

//+kubebuilder:object:root=true

// MaxScaleList contains a list of MaxScale
type MaxScaleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MaxScale `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MaxScale{}, &MaxScaleList{})
}
