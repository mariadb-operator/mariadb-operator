package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MaxScaleSpec defines the desired state of MaxScale
type MaxScaleSpec struct{}

// MaxScaleStatus defines the observed state of MaxScale
type MaxScaleStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MaxScale is the Schema for the maxscales API
type MaxScale struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MaxScaleSpec   `json:"spec,omitempty"`
	Status MaxScaleStatus `json:"status,omitempty"`
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
