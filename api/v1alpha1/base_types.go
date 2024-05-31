package v1alpha1

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/webhook"
	cron "github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var (
	inmutableWebhook = webhook.NewInmutableWebhook(
		webhook.WithTagName("webhook"),
	)
	cronParser = cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)
)

// MariaDBRef is a reference to a MariaDB object.
type MariaDBRef struct {
	// ObjectReference is a reference to a object.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	corev1.ObjectReference `json:",inline"`
	// WaitForIt indicates whether the controller using this reference should wait for MariaDB to be ready.
	// +optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	WaitForIt bool `json:"waitForIt"`
}

// SecretTemplate defines a template to customize Secret objects.
type SecretTemplate struct {
	// Metadata to be added to the Secret object.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Metadata *Metadata `json:"metadata,omitempty"`
	// Key to be used in the Secret.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Key *string `json:"key,omitempty"`
	// Format to be used in the Secret.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Format *string `json:"format,omitempty"`
	// UsernameKey to be used in the Secret.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	UsernameKey *string `json:"usernameKey,omitempty"`
	// PasswordKey to be used in the Secret.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PasswordKey *string `json:"passwordKey,omitempty"`
	// HostKey to be used in the Secret.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	HostKey *string `json:"hostKey,omitempty"`
	// PortKey to be used in the Secret.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PortKey *string `json:"portKey,omitempty"`
	// DatabaseKey to be used in the Secret.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	DatabaseKey *string `json:"databaseKey,omitempty"`
}

// ContainerTemplate defines a template to configure Container objects.
type ContainerTemplate struct {
	// Command to be used in the Container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Command []string `json:"command,omitempty"`
	// Args to be used in the Container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Args []string `json:"args,omitempty"`
	// Env represents the environment variables to be injected in a container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Env []corev1.EnvVar `json:"env,omitempty"`
	// EnvFrom represents the references (via ConfigMap and Secrets) to environment variables to be injected in the container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
	// VolumeMounts to be used in the Container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty" webhook:"inmutable"`
	// LivenessProbe to be used in the Container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`
	// ReadinessProbe to be used in the Container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
	// Resouces describes the compute resource requirements.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// SecurityContext holds security configuration that will be applied to a container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// JobContainerTemplate defines a template to configure Container objects that run in a Job.
type JobContainerTemplate struct {
	// Args to be used in the Container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Args []string `json:"args,omitempty"`
	// Resouces describes the compute resource requirements.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// SecurityContext holds security configuration that will be applied to a container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// FromContainerTemplate sets the ContainerTemplate fields in the current JobContainerTemplate.
func (j *JobContainerTemplate) FromContainerTemplate(ctpl *ContainerTemplate) {
	if j.Args == nil {
		j.Args = ctpl.Args
	}
	if j.Resources == nil {
		j.Resources = ctpl.Resources
	}
	if j.SecurityContext == nil {
		j.SecurityContext = ctpl.SecurityContext
	}
}

// Container object definition.
type Container struct {
	// ContainerTemplate defines a template to configure Container objects.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerTemplate `json:",inline"`
	// Image name to be used by the MariaDB instances. The supported format is `<image>:<tag>`.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Image string `json:"image"`
	// ImagePullPolicy is the image pull policy. One of `Always`, `Never` or `IfNotPresent`. If not defined, it defaults to `IfNotPresent`.
	// +optional
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:imagePullPolicy","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// Job defines a Job used to be used with MariaDB.
type Job struct {
	// Metadata defines additional metadata for the bootstrap Jobs.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Metadata *Metadata `json:"metadata,omitempty"`
	// Affinity to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Affinity *AffinityConfig `json:"affinity,omitempty"`
	// Resouces describes the compute resource requirements.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// Args to be used in the Container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Args []string `json:"args,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (j *Job) SetDefaults(mariadbObjMeta metav1.ObjectMeta) {
	if j.Affinity != nil {
		j.Affinity.SetDefaults(mariadbObjMeta.Name)
	}
}

// AffinityConfig defines policies to schedule Pods in Nodes.
type AffinityConfig struct {
	// Affinity to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	corev1.Affinity `json:",inline"`
	// AntiAffinityEnabled configures PodAntiAffinity so each Pod is scheduled in a different Node, enabling HA.
	// Make sure you have at least as many Nodes available as the replicas to not end up with unscheduled Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	AntiAffinityEnabled *bool `json:"antiAffinityEnabled,omitempty" webhook:"inmutable"`
}

// SetDefaults sets reasonable defaults.
func (a *AffinityConfig) SetDefaults(antiAffinityInstances ...string) {
	if ptr.Deref(a.AntiAffinityEnabled, false) && reflect.ValueOf(a.Affinity).IsZero() && len(antiAffinityInstances) > 0 {
		a.Affinity = corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app.kubernetes.io/instance",
									Operator: metav1.LabelSelectorOpIn,
									Values:   antiAffinityInstances,
								},
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		}
	}
}

// PodTemplate defines a template to configure Container objects.
type PodTemplate struct {
	// PodMetadata defines extra metadata for the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodMetadata *Metadata `json:"podMetadata,omitempty"`
	// ImagePullSecrets is the list of pull Secrets to be used to pull the image.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" webhook:"inmutable"`
	// InitContainers to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InitContainers []Container `json:"initContainers,omitempty"`
	// SidecarContainers to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SidecarContainers []Container `json:"sidecarContainers,omitempty"`
	// SecurityContext holds pod-level security attributes and common container settings.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to be used by the Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServiceAccountName *string `json:"serviceAccountName,omitempty" webhook:"inmutableinit"`
	// Affinity to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Affinity *AffinityConfig `json:"affinity,omitempty"`
	// NodeSelector to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// Volumes to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Volumes []corev1.Volume `json:"volumes,omitempty" webhook:"inmutable"`
	// PriorityClassName to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PriorityClassName *string `json:"priorityClassName,omitempty" webhook:"inmutable"`
	// TopologySpreadConstraints to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (p *PodTemplate) SetDefaults(objMeta metav1.ObjectMeta) {
	if p.ServiceAccountName == nil {
		p.ServiceAccountName = ptr.To(p.ServiceAccountKey(objMeta).Name)
	}
	if p.Affinity != nil {
		p.Affinity.SetDefaults(objMeta.Name)
	}
}

// ServiceAccountKey defines the key for the ServiceAccount object.
func (p *PodTemplate) ServiceAccountKey(objMeta metav1.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{
		Name:      ptr.Deref(p.ServiceAccountName, objMeta.Name),
		Namespace: objMeta.Namespace,
	}
}

// JobPodTemplate defines a template to configure Container objects that run in a Job.
type JobPodTemplate struct {
	// PodMetadata defines extra metadata for the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodMetadata *Metadata `json:"podMetadata,omitempty"`
	// ImagePullSecrets is the list of pull Secrets to be used to pull the image.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" webhook:"inmutable"`
	// SecurityContext holds pod-level security attributes and common container settings.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to be used by the Pods.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServiceAccountName *string `json:"serviceAccountName,omitempty" webhook:"inmutableinit"`
	// Affinity to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Affinity *AffinityConfig `json:"affinity,omitempty"`
	// NodeSelector to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// PriorityClassName to be used in the Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	PriorityClassName *string `json:"priorityClassName,omitempty" webhook:"inmutable"`
}

// FromPodTemplate sets the PodTemplate fields in the current JobPodTemplate.
func (j *JobPodTemplate) FromPodTemplate(ptpl *PodTemplate) {
	if j.PodMetadata == nil {
		j.PodMetadata = ptpl.PodMetadata
	}
	if j.ImagePullSecrets == nil {
		j.ImagePullSecrets = ptpl.ImagePullSecrets
	}
	if j.PodSecurityContext == nil {
		j.PodSecurityContext = ptpl.PodSecurityContext
	}
	if j.ServiceAccountName == nil {
		j.ServiceAccountName = ptpl.ServiceAccountName
	}
	if j.Affinity == nil {
		j.Affinity = ptpl.Affinity
	}
	if j.NodeSelector == nil {
		j.NodeSelector = ptpl.NodeSelector
	}
	if j.Tolerations == nil {
		j.Tolerations = ptpl.Tolerations
	}
	if j.PriorityClassName == nil {
		j.PriorityClassName = ptpl.PriorityClassName
	}
}

// SetDefaults sets reasonable defaults.
func (p *JobPodTemplate) SetDefaults(objMeta, mariadbObjMeta metav1.ObjectMeta) {
	if p.ServiceAccountName == nil {
		p.ServiceAccountName = ptr.To(p.ServiceAccountKey(objMeta).Name)
	}
	if p.Affinity != nil {
		p.Affinity.SetDefaults(mariadbObjMeta.Name)
	}
}

// ServiceAccountKey defines the key for the ServiceAccount object.
func (p *JobPodTemplate) ServiceAccountKey(objMeta metav1.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{
		Name:      ptr.Deref(p.ServiceAccountName, objMeta.Name),
		Namespace: objMeta.Namespace,
	}
}

// VolumeClaimTemplate defines a template to customize PVC objects.
type VolumeClaimTemplate struct {
	// PersistentVolumeClaimSpec is the specification of a PVC.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	corev1.PersistentVolumeClaimSpec `json:",inline"`
	// Metadata to be added to the PVC metadata.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Metadata *Metadata `json:"metadata,omitempty"`
}

// ServiceTemplate defines a template to customize Service objects.
type ServiceTemplate struct {
	// Type is the Service type. One of `ClusterIP`, `NodePort` or `LoadBalancer`. If not defined, it defaults to `ClusterIP`.
	// +optional
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Type corev1.ServiceType `json:"type,omitempty"`
	// Metadata to be added to the Service metadata.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Metadata *Metadata `json:"metadata,omitempty"`
	// LoadBalancerIP Service field.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	LoadBalancerIP *string `json:"loadBalancerIP,omitempty"`
	// LoadBalancerSourceRanges Service field.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`
	// ExternalTrafficPolicy Service field.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty"`
	// SessionAffinity Service field.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SessionAffinity *corev1.ServiceAffinity `json:"sessionAffinity,omitempty"`
	// AllocateLoadBalancerNodePorts Service field.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	AllocateLoadBalancerNodePorts *bool `json:"allocateLoadBalancerNodePorts,omitempty"`
}

// PodDisruptionBudget is the Pod availability bundget for a MariaDB
type PodDisruptionBudget struct {
	// MinAvailable defines the number of minimum available Pods.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`
	// MaxUnavailable defines the number of maximum unavailable Pods.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

func (p *PodDisruptionBudget) Validate() error {
	if p.MinAvailable != nil && p.MaxUnavailable == nil {
		return nil
	}
	if p.MinAvailable == nil && p.MaxUnavailable != nil {
		return nil
	}
	return errors.New("either minAvailable or maxUnavailable must be specified")
}

// HealthCheck defines intervals for performing health checks.
type HealthCheck struct {
	// Interval used to perform health checks.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Interval *metav1.Duration `json:"interval,omitempty"`
	// RetryInterval is the interval used to perform health check retries.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	RetryInterval *metav1.Duration `json:"retryInterval,omitempty"`
}

// ConnectionTemplate defines a template to customize Connection objects.
type ConnectionTemplate struct {
	// SecretName to be used in the Connection.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SecretName *string `json:"secretName,omitempty" webhook:"inmutableinit"`
	// SecretTemplate to be used in the Connection.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SecretTemplate *SecretTemplate `json:"secretTemplate,omitempty"`
	// HealthCheck to be used in the Connection.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`
	// Params to be used in the Connection.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Params map[string]string `json:"params,omitempty"`
	// ServiceName to be used in the Connection.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ServiceName *string `json:"serviceName,omitempty"`
	// Port to connect to. If not provided, it defaults to the MariaDB port or to the first MaxScale listener.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number","urn:alm:descriptor:com.tectonic.ui:advanced"}
	Port int32 `json:"port,omitempty"`
}

// SQLTemplate defines a template to customize SQL objects.
type SQLTemplate struct {
	// RequeueInterval is used to perform requeue reconciliations.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	RequeueInterval *metav1.Duration `json:"requeueInterval,omitempty"`
	// RetryInterval is the interval used to perform retries.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	RetryInterval *metav1.Duration `json:"retryInterval,omitempty"`
}

type TLS struct {
	// Enabled is a flag to enable TLS.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`
	// CASecretKeyRef is a reference to a Secret key containing a CA bundle in PEM format used to establish TLS connections with S3.
	// By default, the system trust chain will be used, but you can use this field to add more CAs to the bundle.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	CASecretKeyRef *corev1.SecretKeySelector `json:"caSecretKeyRef,omitempty"`
}

type S3 struct {
	// Bucket is the name Name of the bucket to store backups.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Bucket string `json:"bucket" webhook:"inmutable"`
	// Endpoint is the S3 API endpoint without scheme.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Endpoint string `json:"endpoint" webhook:"inmutable"`
	// Region is the S3 region name to use.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Region string `json:"region" webhook:"inmutable"`
	// Prefix indicates a folder/subfolder in the bucket. For example: mariadb/ or mariadb/backups. A trailing slash '/' is added if not provided.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Prefix string `json:"prefix" webhook:"inmutable"`
	// AccessKeyIdSecretKeyRef is a reference to a Secret key containing the S3 access key id.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AccessKeyIdSecretKeyRef corev1.SecretKeySelector `json:"accessKeyIdSecretKeyRef"`
	// AccessKeyIdSecretKeyRef is a reference to a Secret key containing the S3 secret key.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SecretAccessKeySecretKeyRef corev1.SecretKeySelector `json:"secretAccessKeySecretKeyRef"`
	// SessionTokenSecretKeyRef is a reference to a Secret key containing the S3 session token.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	SessionTokenSecretKeyRef *corev1.SecretKeySelector `json:"sessionTokenSecretKeyRef,omitempty"`
	// TLS provides the configuration required to establish TLS connections with S3.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	TLS *TLS `json:"tls,omitempty"`
}

// Metadata defines the metadata to added to resources.
type Metadata struct {
	// Labels to be added to children resources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations to be added to children resources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Annotations map[string]string `json:"annotations,omitempty"`
}

// MergeMetadata merges multiple Metadta instances into one
func MergeMetadata(metas ...*Metadata) *Metadata {
	meta := Metadata{
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}
	for _, m := range metas {
		if m == nil {
			continue
		}
		for k, v := range m.Labels {
			meta.Labels[k] = v
		}
		for k, v := range m.Annotations {
			meta.Annotations[k] = v
		}
	}
	return &meta
}

// RestoreSource defines a source for restoring a MariaDB.
type RestoreSource struct {
	// BackupRef is a reference to a Backup object. It has priority over S3 and Volume.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	BackupRef *corev1.LocalObjectReference `json:"backupRef,omitempty" webhook:"inmutableinit"`
	// S3 defines the configuration to restore backups from a S3 compatible storage. It has priority over Volume.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	S3 *S3 `json:"s3,omitempty" webhook:"inmutableinit"`
	// Volume is a Kubernetes Volume object that contains a backup.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Volume *corev1.VolumeSource `json:"volume,omitempty" webhook:"inmutableinit"`
	// TargetRecoveryTime is a RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.
	// It is used to determine the closest restoration source in time.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	TargetRecoveryTime *metav1.Time `json:"targetRecoveryTime,omitempty" webhook:"inmutable"`
}

func (r *RestoreSource) Validate() error {
	if r.BackupRef == nil && r.S3 == nil && r.Volume == nil {
		return errors.New("unable to determine restore source")
	}
	return nil
}

func (r *RestoreSource) IsDefaulted() bool {
	return r.Volume != nil
}

func (r *RestoreSource) SetDefaults() {
	if r.S3 != nil {
		r.Volume = &corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	}
}

func (r *RestoreSource) SetDefaultsWithBackup(backup *Backup) error {
	volume, err := backup.Volume()
	if err != nil {
		return fmt.Errorf("error getting backup volume: %v", err)
	}
	r.Volume = volume
	r.S3 = backup.Spec.Storage.S3
	return nil
}

func (r *RestoreSource) TargetRecoveryTimeOrDefault() time.Time {
	if r.TargetRecoveryTime != nil {
		return r.TargetRecoveryTime.Time
	}
	return time.Now()
}

// Schedule contains parameters to define a schedule
type Schedule struct {
	// Cron is a cron expression that defines the schedule.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Cron string `json:"cron"`
	// Suspend defines whether the schedule is active or not.
	// +optional
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	Suspend bool `json:"suspend"`
}

func (s *Schedule) Validate() error {
	_, err := cronParser.Parse(s.Cron)
	return err
}

// GeneratedSecretKeyRef defines a reference to a Secret that can be automatically generated by mariadb-operator if needed.
type GeneratedSecretKeyRef struct {
	// SecretKeySelector is a reference to a Secret key.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	corev1.SecretKeySelector `json:",inline"`
	// Generate indicates whether the Secret should be generated if the Secret referenced is not present.
	// +optional
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Generate bool `json:"generate,omitempty"`
}
