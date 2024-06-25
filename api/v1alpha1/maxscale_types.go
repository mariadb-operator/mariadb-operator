package v1alpha1

import (
	"errors"
	"fmt"
	"time"

	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	mxsstate "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/state"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Maintenance bool `json:"maintenance,omitempty"`
	// Params defines extra parameters to pass to the server.
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#server_1.
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
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
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

// Validate determines whether a MonitorModule is valid.
func (m MonitorModule) Validate() error {
	switch m {
	case MonitorModuleMariadb, MonitorModuleGalera:
		return nil
	}
	return fmt.Errorf("unsupported value: '%v'", m)
}

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
	// SuspendTemplate defines how a resource can be suspended.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SuspendTemplate `json:",inline"`
	// Name is the identifier of the monitor. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Module is the module to use to monitor MariaDB servers. It is mandatory when no MariaDB reference is provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Module MonitorModule `json:"module" webhook:"inmutableinit"`
	// Interval used to monitor MariaDB servers. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Interval metav1.Duration `json:"interval,omitempty"`
	// CooperativeMonitoring enables coordination between multiple MaxScale instances running monitors. It is defaulted when HA is enabled.
	// +optional
	// +kubebuilder:validation:Enum=majority_of_all;majority_of_running
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	CooperativeMonitoring *CooperativeMonitoring `json:"cooperativeMonitoring,omitempty"`
	// Params defines extra parameters to pass to the monitor.
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-common-monitor-parameters/.
	// Monitor specific parameter are also suported:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-galera-monitor/#galera-monitor-optional-parameters.
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-monitor/#configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Params map[string]string `json:"params,omitempty"`
}

// SetCondition sets a status condition to MaxScale
func (m *MaxScaleMonitor) SetDefaults(mxs *MaxScale) {
	if m.Name == "" && m.Module != "" {
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
	// SuspendTemplate defines how a resource can be suspended.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SuspendTemplate `json:",inline"`
	// Name is the identifier of the listener. It is defaulted if not provided
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`
	// Port is the network port where the MaxScale server will listen.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	Port int32 `json:"port,omitempty" webhook:"inmutable"`
	// Protocol is the MaxScale protocol to use when communicating with the client. If not provided, it defaults to MariaDBProtocol.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Protocol string `json:"protocol,omitempty"`
	// Params defines extra parameters to pass to the listener.
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#listener_1.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
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
	// SuspendTemplate defines how a resource can be suspended.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
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
	Listener MaxScaleListener `json:"listener"`
	// Params defines extra parameters to pass to the service.
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#service_1.
	// Router specific parameter are also suported:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-readwritesplit/#configuration.
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-readconnroute/#configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
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
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
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
	// Any parameter supported by MaxScale may be specified here. See reference:
	// https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#global-settings.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Params map[string]string `json:"params,omitempty"`
	// VolumeClaimTemplate provides a template to define the PVCs for storing MaxScale runtime configuration files. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	VolumeClaimTemplate VolumeClaimTemplate `json:"volumeClaimTemplate"`
	// Sync defines how to replicate configuration across MaxScale replicas. It is defaulted when HA is enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Sync *MaxScaleConfigSync `json:"sync,omitempty"`
}

func (m *MaxScaleConfig) SetDefaults(mxs *MaxScale) {
	if m.VolumeClaimTemplate.Resources.Requests == nil {
		m.VolumeClaimTemplate.Resources.Requests = make(corev1.ResourceList)
	}

	if _, exit := m.VolumeClaimTemplate.Resources.Requests["storage"]; !exit {
		m.VolumeClaimTemplate.Resources.Requests["storage"] = resource.MustParse("100Mi")
	}

	if len(m.VolumeClaimTemplate.AccessModes) == 0 {
		m.VolumeClaimTemplate.AccessModes = append(m.VolumeClaimTemplate.AccessModes, corev1.ReadWriteOnce)
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
	// Generate  defies whether the operator should generate users and grants for MaxScale to work.
	// It only supports MariaDBs specified via spec.mariaDbRef.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Generate *bool `json:"generate,omitempty" webhook:"inmutableinit"`
	// AdminUsername is an admin username to call the admin REST API. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	AdminUsername string `json:"adminUsername,omitempty" webhook:"inmutableinit"`
	// AdminPasswordSecretKeyRef is Secret key reference to the admin password to call the admin REST API. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	AdminPasswordSecretKeyRef GeneratedSecretKeyRef `json:"adminPasswordSecretKeyRef,omitempty" webhook:"inmutableinit"`
	// DeleteDefaultAdmin determines whether the default admin user should be deleted after the initial configuration. If not provided, it defaults to true.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	DeleteDefaultAdmin *bool `json:"deleteDefaultAdmin,omitempty" webhook:"inmutableinit"`
	// MetricsUsername is an metrics username to call the REST API. It is defaulted if metrics are enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	MetricsUsername string `json:"metricsUsername,omitempty" webhook:"inmutableinit"`
	// MetricsPasswordSecretKeyRef is Secret key reference to the metrics password to call the admib REST API. It is defaulted if metrics are enabled.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	MetricsPasswordSecretKeyRef GeneratedSecretKeyRef `json:"metricsPasswordSecretKeyRef,omitempty"`
	// ClientUsername is the user to connect to MaxScale. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientUsername string `json:"clientUsername,omitempty" webhook:"inmutableinit"`
	// ClientPasswordSecretKeyRef is Secret key reference to the password to connect to MaxScale. It is defaulted if not provided.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientPasswordSecretKeyRef GeneratedSecretKeyRef `json:"clientPasswordSecretKeyRef,omitempty"`
	// ClientMaxConnections defines the maximum number of connections that the client can establish.
	// If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.
	// It defaults to 30 times the number of MaxScale replicas.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClientMaxConnections int32 `json:"clientMaxConnections,omitempty" webhook:"inmutableinit"`
	// ServerUsername is the user used by MaxScale to connect to MariaDB server. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerUsername string `json:"serverUsername,omitempty" webhook:"inmutableinit"`
	// ServerPasswordSecretKeyRef is Secret key reference to the password used by MaxScale to connect to MariaDB server. It is defaulted if not provided.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerPasswordSecretKeyRef GeneratedSecretKeyRef `json:"serverPasswordSecretKeyRef,omitempty"`
	// ServerMaxConnections defines the maximum number of connections that the server can establish.
	// If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.
	// It defaults to 30 times the number of MaxScale replicas.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServerMaxConnections int32 `json:"serverMaxConnections,omitempty" webhook:"inmutableinit"`
	// MonitorUsername is the user used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	MonitorUsername string `json:"monitorUsername,omitempty" webhook:"inmutableinit"`
	// MonitorPasswordSecretKeyRef is Secret key reference to the password used by MaxScale monitor to connect to MariaDB server. It is defaulted if not provided.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	MonitorPasswordSecretKeyRef GeneratedSecretKeyRef `json:"monitorPasswordSecretKeyRef,omitempty"`
	// MonitorMaxConnections defines the maximum number of connections that the monitor can establish.
	// If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.
	// It defaults to 30 times the number of MaxScale replicas.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	MonitorMaxConnections int32 `json:"monitorMaxConnections,omitempty" webhook:"inmutableinit"`
	// MonitoSyncUsernamerUsername is the user used by MaxScale config sync to connect to MariaDB server. It is defaulted when HA is enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SyncUsername *string `json:"syncUsername,omitempty" webhook:"inmutableinit"`
	// SyncPasswordSecretKeyRef is Secret key reference to the password used by MaxScale config to connect to MariaDB server. It is defaulted when HA is enabled.
	// If the referred Secret is labeled with "k8s.mariadb.com/watch", updates may be performed to the Secret in order to update the password.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SyncPasswordSecretKeyRef *GeneratedSecretKeyRef `json:"syncPasswordSecretKeyRef,omitempty"`
	// SyncMaxConnections defines the maximum number of connections that the sync can establish.
	// If HA is enabled, make sure to increase this value, as more MaxScale replicas implies more connections.
	// It defaults to 30 times the number of MaxScale replicas.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	SyncMaxConnections *int32 `json:"syncMaxConnections,omitempty" webhook:"inmutableinit"`
}

// SetDefaults sets default values.
func (m *MaxScaleAuth) SetDefaults(mxs *MaxScale) {
	if mxs.Spec.MariaDBRef != nil && m.Generate == nil {
		m.Generate = ptr.To(true)
	}
	if m.AdminUsername == "" {
		m.AdminUsername = "mariadb-operator"
	}
	if m.AdminPasswordSecretKeyRef == (GeneratedSecretKeyRef{}) {
		m.AdminPasswordSecretKeyRef = mxs.AdminPasswordSecretKeyRef()
	}
	if m.DeleteDefaultAdmin == nil {
		m.DeleteDefaultAdmin = ptr.To(true)
	}

	metrics := ptr.Deref(mxs.Spec.Metrics, MaxScaleMetrics{})
	if metrics.Enabled {
		if m.MetricsUsername == "" {
			m.MetricsUsername = "metrics"
		}
		if m.MetricsPasswordSecretKeyRef == (GeneratedSecretKeyRef{}) {
			m.MetricsPasswordSecretKeyRef = mxs.MetricsPasswordSecretKeyRef()
		}
	}

	if m.ClientUsername == "" {
		m.ClientUsername = mxs.AuthClientUserKey().Name
	}
	if m.ClientPasswordSecretKeyRef == (GeneratedSecretKeyRef{}) {
		m.ClientPasswordSecretKeyRef = mxs.AuthClientPasswordSecretKeyRef()
	}
	if m.ClientMaxConnections == 0 {
		m.ClientMaxConnections = mxs.defaultConnections()
	}

	if m.ServerUsername == "" {
		m.ServerUsername = mxs.AuthServerUserKey().Name
	}
	if m.ServerPasswordSecretKeyRef == (GeneratedSecretKeyRef{}) {
		m.ServerPasswordSecretKeyRef = mxs.AuthServerPasswordSecretKeyRef()
	}
	if m.ServerMaxConnections == 0 {
		m.ServerMaxConnections = mxs.defaultConnections()
	}

	if m.MonitorUsername == "" {
		m.MonitorUsername = mxs.AuthMonitorUserKey().Name
	}
	if m.MonitorPasswordSecretKeyRef == (GeneratedSecretKeyRef{}) {
		m.MonitorPasswordSecretKeyRef = mxs.AuthMonitorPasswordSecretKeyRef()
	}
	if m.MonitorMaxConnections == 0 {
		m.MonitorMaxConnections = mxs.defaultConnections()
	}

	if mxs.IsHAEnabled() {
		if m.SyncUsername == nil {
			m.SyncUsername = ptr.To(mxs.AuthSyncUserKey().Name)
		}
		if m.SyncPasswordSecretKeyRef == nil {
			m.SyncPasswordSecretKeyRef = ptr.To(mxs.AuthSyncPasswordSecretKeyRef())
		}
		if m.SyncMaxConnections == nil {
			m.SyncMaxConnections = ptr.To(mxs.defaultConnections())
		}
	}
}

// MaxScaleMetrics defines the metrics for a Maxscale.
type MaxScaleMetrics struct {
	// Enabled is a flag to enable Metrics
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
	// Exporter defines the metrics exporter container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Exporter Exporter `json:"exporter"`
	// ServiceMonitor defines the ServiceMonior object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServiceMonitor ServiceMonitor `json:"serviceMonitor"`
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
	// MariaDBRef is a reference to the MariaDB that MaxScale points to. It is used to initialize the servers field.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MariaDBRef *MariaDBRef `json:"mariaDbRef,omitempty" webhook:"inmutable"`
	// Servers are the MariaDB servers to forward traffic to. It is required if 'spec.mariaDbRef' is not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Servers []MaxScaleServer `json:"servers"`
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
	// InheritMetadata defines the metadata to be inherited by children resources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InheritMetadata *Metadata `json:"inheritMetadata,omitempty"`
	// Services define how the traffic is forwarded to the MariaDB servers. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Services []MaxScaleService `json:"services,omitempty"`
	// Monitor monitors MariaDB server instances. It is required if 'spec.mariaDbRef' is not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Monitor MaxScaleMonitor `json:"monitor,omitempty"`
	// Admin configures the admin REST API and GUI.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Admin MaxScaleAdmin `json:"admin,omitempty"`
	// Config defines the MaxScale configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Config MaxScaleConfig `json:"config,omitempty"`
	// Auth defines the credentials required for MaxScale to connect to MariaDB.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Auth MaxScaleAuth `json:"auth,omitempty"`
	// Metrics configures metrics and how to scrape them.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Metrics *MaxScaleMetrics `json:"metrics,omitempty"`
	// Connection provides a template to define the Connection for MaxScale.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Connection *ConnectionTemplate `json:"connection,omitempty"`
	// Replicas indicates the number of desired instances.
	// +kubebuilder:default=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// PodDisruptionBudget defines the budget for replica availability.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodDisruptionBudget *PodDisruptionBudget `json:"podDisruptionBudget,omitempty"`
	// UpdateStrategy defines the update strategy for the StatefulSet object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:updateStrategy","urn:alm:descriptor:com.tectonic.ui:advanced"}
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	// KubernetesService defines a template for a Kubernetes Service object to connect to MaxScale.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	KubernetesService *ServiceTemplate `json:"kubernetesService,omitempty"`
	// GuiKubernetesService defines a template for a Kubernetes Service object to connect to MaxScale's GUI.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	GuiKubernetesService *ServiceTemplate `json:"guiKubernetesService,omitempty"`
	// RequeueInterval is used to perform requeue reconciliations. If not defined, it defaults to 10s.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	RequeueInterval *metav1.Duration `json:"requeueInterval,omitempty"`
}

// MaxScaleAPIStatus is the state of the servers in the MaxScale API.
type MaxScaleServerStatus struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// IsMaster indicates whether the current server is in Master state.
func (s *MaxScaleServerStatus) IsMaster() bool {
	return mxsstate.IsMaster(s.State)
}

// IsReady indicates whether the current server is in ready state.
func (s *MaxScaleServerStatus) IsReady() bool {
	return mxsstate.IsReady(s.State)
}

// InMaintenance indicates whether the current server is in maintenance state.
func (s *MaxScaleServerStatus) InMaintenance() bool {
	return mxsstate.InMaintenance(s.State)
}

// MaxScaleResourceStatus indicates whether the resource is in a given state.
type MaxScaleResourceStatus struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type MaxScaleConfigSyncStatus struct {
	MaxScaleVersion int `json:"maxScaleVersion"`
	DatabaseVersion int `json:"databaseVersion"`
}

// MaxScaleStatus defines the observed state of MaxScale
type MaxScaleStatus struct {
	// Conditions for the MaxScale object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Replicas indicates the number of current instances.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// PrimaryServer is the primary server in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	PrimaryServer *string `json:"primaryServer,omitempty"`
	// Servers is the state of the servers in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Servers []MaxScaleServerStatus `json:"servers,omitempty"`
	// Monitor is the state of the monitor in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Monitor *MaxScaleResourceStatus `json:"monitor,omitempty"`
	// Services is the state of the services in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Services []MaxScaleResourceStatus `json:"services,omitempty"`
	// Listeners is the state of the listeners in the MaxScale API.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Listeners []MaxScaleResourceStatus `json:"listeners,omitempty"`
	// ConfigSync is the state of config sync.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ConfigSync *MaxScaleConfigSyncStatus `json:"configSync,omitempty"`
}

// SetCondition sets a status condition to MaxScale
func (s *MaxScaleStatus) SetCondition(condition metav1.Condition) {
	if s.Conditions == nil {
		s.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&s.Conditions, condition)
}

// GetPrimaryServer obtains the current primary server.
func (s *MaxScaleStatus) GetPrimaryServer() *string {
	for _, srv := range s.Servers {
		if srv.IsMaster() {
			return &srv.Name
		}
	}
	return nil
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=mxs
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Primary Server",type="string",JSONPath=".status.primaryServer"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{MaxScale,v1alpha1},{User,v1alpha1},{Grant,v1alpha1},{Connection,v1alpha1},{Event,v1},{Service,v1},{Secret,v1},{ServiceAccount,v1},{StatefulSet,v1},{Deployment,v1},{PodDisruptionBudget,v1}}

// MaxScale is the Schema for the maxscales API. It is used to define MaxScale clusters.
type MaxScale struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MaxScaleSpec   `json:"spec,omitempty"`
	Status MaxScaleStatus `json:"status,omitempty"`
}

// SetDefaults sets default values.
func (m *MaxScale) SetDefaults(env *environment.OperatorEnv, mariadb *MariaDB) {
	if m.Spec.Image == "" {
		m.Spec.Image = env.RelatedMaxscaleImage
	}
	if m.Spec.RequeueInterval == nil {
		m.Spec.RequeueInterval = &metav1.Duration{Duration: 10 * time.Second}
	}
	for i := range m.Spec.Servers {
		m.Spec.Servers[i].SetDefaults()
	}
	if len(m.Spec.Services) == 0 {
		m.Spec.Services = []MaxScaleService{
			{
				Name:   "rw-router",
				Router: ServiceRouterReadWriteSplit,
				Listener: MaxScaleListener{
					Port: 3306,
				},
				Params: map[string]string{
					"transaction_replay":  "true",
					"master_accept_reads": "true",
				},
			},
			{
				Name:   "rconn-master-router",
				Router: ServiceRouterReadConnRoute,
				Listener: MaxScaleListener{
					Port: 3307,
				},
				Params: map[string]string{
					"router_options":      "master",
					"master_accept_reads": "true",
				},
			},
			{
				Name:   "rconn-slave-router",
				Router: ServiceRouterReadConnRoute,
				Listener: MaxScaleListener{
					Port: 3308,
				},
				Params: map[string]string{
					"router_options": "slave",
				},
			},
		}
	}
	for i := range m.Spec.Services {
		m.Spec.Services[i].SetDefaults()
	}
	m.Spec.Monitor.SetDefaults(m)
	m.Spec.Admin.SetDefaults(m)
	m.Spec.Config.SetDefaults(m)
	m.Spec.Auth.SetDefaults(m)

	antiAffinityInstances := m.getAntiAffinityInstances(mariadb)

	if m.AreMetricsEnabled() {
		if m.Spec.Metrics.Exporter.Image == "" {
			m.Spec.Metrics.Exporter.Image = env.RelatedExporterMaxscaleImage
		}
		if m.Spec.Metrics.Exporter.Port == 0 {
			m.Spec.Metrics.Exporter.Port = 9105
		}
		if m.Spec.Metrics.Exporter.Affinity != nil {
			m.Spec.Metrics.Exporter.Affinity.SetDefaults(antiAffinityInstances...)
		}
	}

	if m.Spec.Affinity != nil {
		m.Spec.Affinity.SetDefaults(antiAffinityInstances...)
	}

	m.Spec.PodTemplate.SetDefaults(m.ObjectMeta)
}

func (m *MaxScale) getAntiAffinityInstances(mariadb *MariaDB) []string {
	instances := []string{m.ObjectMeta.Name}
	if mariadb != nil {
		instances = append(instances, mariadb.ObjectMeta.Name)
	}
	return instances
}

// IsBeingDeleted indicates that MaxScale has been marked for deletion
func (m *MaxScale) IsBeingDeleted() bool {
	return !m.DeletionTimestamp.IsZero()
}

// IsReady indicates whether the Maxscale instance is ready.
func (m *MaxScale) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReady)
}

// IsHAEnabled indicated whether high availability is enabled.
func (m *MaxScale) IsHAEnabled() bool {
	return m.Spec.Replicas > 1
}

// AreMetricsEnabled indicates whether the MariaDB instance has metrics enabled
func (m *MaxScale) AreMetricsEnabled() bool {
	return ptr.Deref(m.Spec.Metrics, MaxScaleMetrics{}).Enabled
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

// DefaultPort returns the default port.
func (m *MaxScale) DefaultPort() (*int32, error) {
	if len(m.Spec.Services) == 0 {
		return nil, errors.New("port not found")
	}
	return &m.Spec.Services[0].Listener.Port, nil
}

func (m *MaxScale) apiUrlWithAddress(addr string) string {
	return fmt.Sprintf("http://%s:%d", addr, m.Spec.Admin.Port)
}

func (m *MaxScale) defaultConnections() int32 {
	if m.Spec.Replicas > 0 {
		return m.Spec.Replicas * 30
	}
	return 30
}

// MaxScaleMetricsPasswordSecretFieldPath is the path related to the metrics password Secret field.
const MaxScaleMetricsPasswordSecretFieldPath = ".spec.auth.metricsPasswordSecretKeyRef.name"

// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (m *MaxScale) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case MaxScaleMetricsPasswordSecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.AreMetricsEnabled() && maxscale.Spec.Auth.MetricsPasswordSecretKeyRef.Name != "" {
				return []string{maxscale.Spec.Auth.MetricsPasswordSecretKeyRef.Name}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

//+kubebuilder:object:root=true

// MaxScaleList contains a list of MaxScale
type MaxScaleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MaxScale `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *MaxScaleList) ListItems() []client.Object {
	items := make([]client.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&MaxScale{}, &MaxScaleList{})
}
