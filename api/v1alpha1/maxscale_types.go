package v1alpha1

import (
	"fmt"
	"reflect"

	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// MaxScaleAdmin configures the admin REST API and GUI.
type MaxScaleAdmin struct {
	// Port where the admin REST API will be exposed.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Port int `json:"port"`
	// Username is an admin username to call the REST API. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Username string `json:"username,omitempty"`
	// PasswordSecretKeyRef is Secret key reference to the admin password to call the REST API. It is defaulted if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PasswordSecretKeyRef corev1.SecretKeySelector `json:"passwordSecretKeyRef,omitempty"`
	// GuiEnabled indicates whether the admin GUI should be enabled.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	GuiEnabled *bool `json:"guiEnabled,omitempty"`
}

func (m *MaxScaleAdmin) SetDefaults(mxs *MaxScale) {
	if m.Port == 0 {
		m.Port = 8989
	}
	if m.Username == "" {
		m.Username = "mariadb-operator"
	}
	if m.PasswordSecretKeyRef == (corev1.SecretKeySelector{}) {
		m.PasswordSecretKeyRef = mxs.AdminPasswordSecretKeyRef()
	}
	if m.GuiEnabled == nil {
		m.GuiEnabled = ptr.To(true)
	}
}

// MaxScaleConfig defines the MaxScale configuration.
type MaxScaleConfig struct {
	// Params is a key value pair of parameters to be used in the MaxScale static configuration file.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Params map[string]string `json:"params,omitempty"`
	// VolumeClaimTemplate provides a template to define the PVCs for storing MaxScale runtime configuration files.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	VolumeClaimTemplate VolumeClaimTemplate `json:"volumeClaimTemplate"`
}

func (m *MaxScaleConfig) SetDefaults() {
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
}

// MaxScaleAuth defines the credentials required for MaxScale to connect to MariaDB
type MaxScaleAuth struct {
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
	// MonitorUsername is the user used by MaxScale monitor to connect to MariaDB server. It is only required if the monitor is enabled and defaulted if not provided
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MonitorUsername string `json:"monitorUsername,omitempty"`
	// MonitorPasswordSecretKeyRef is Secret key reference to the password used by MaxScale monitor to connect to MariaDB server. It is only required if the monitor is enabled and defaulted if not provided
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MonitorPasswordSecretKeyRef corev1.SecretKeySelector `json:"monitorPasswordSecretKeyRef,omitempty"`
}

func (m *MaxScaleAuth) SetDefaults(mxs *MaxScale) {
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
}

// MaxScaleSpec defines the desired state of MaxScale
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
}

// MaxScaleStatus defines the observed state of MaxScale
type MaxScaleStatus struct {
	// Conditions for the Mariadb object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Replicas indicates the number of current instances.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// PrimaryServer is the primary server.
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
// +operator-sdk:csv:customresourcedefinitions:resources={{MaxScale,v1alpha1},{User,v1alpha1},{Grant,v1alpha1},{Event,v1},{Service,v1},{Secret,v1},{StatefulSet,v1},{PodDisruptionBudget,v1}}

// MaxScale is the Schema for the maxscales API
type MaxScale struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MaxScaleSpec   `json:"spec,omitempty"`
	Status MaxScaleStatus `json:"status,omitempty"`
}

func (m *MaxScale) SetDefaults(env *environment.Environment) {
	if m.Spec.Image == "" {
		m.Spec.Image = env.RelatedMaxscaleImage
	}
	m.Spec.Admin.SetDefaults(m)
	m.Spec.Config.SetDefaults()
	m.Spec.Auth.SetDefaults(m)
}

// IsReady indicates whether the Maxscale instance is ready
func (m *MaxScale) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReady)
}

// APIUrl returns the URL of the admin API pointing to the Kubernetes Service
func (m *MaxScale) APIUrl() string {
	fqdn := statefulset.ServiceFQDNWithService(m.ObjectMeta, m.Name)
	return m.apiUrlWithAddress(fqdn)
}

// PodAPIUrl returns the URL of the admin API pointing to a Pod
func (m *MaxScale) PodAPIUrl(podIndex int) string {
	fqdn := statefulset.PodFQDNWithService(m.ObjectMeta, podIndex, m.InternalServiceKey().Name)
	return m.apiUrlWithAddress(fqdn)
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
