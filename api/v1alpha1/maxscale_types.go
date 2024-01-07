package v1alpha1

import (
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	// Replicas indicates the number of desired instances.
	// +kubebuilder:default=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas,omitempty"`
	// PodDisruptionBudget defines the budget for replica availability.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodDisruptionBudget *PodDisruptionBudget `json:"podDisruptionBudget,omitempty"`
	// PodDisruptionBudget defines the update strategy for the Deployment object.
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
	// CurrentPrimary is the primary Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes:Pod"}
	CurrentPrimary *string `json:"currentPrimary,omitempty"`
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
// +kubebuilder:printcolumn:name="Primary",type="string",JSONPath=".status.currentPrimary"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:resources={{MaxScale,v1alpha1},{User,v1alpha1},{Grant,v1alpha1},{Service,v1},{Secret,v1},{Event,v1},{Deployment,v1}{PodDisruptionBudget,v1}}

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
