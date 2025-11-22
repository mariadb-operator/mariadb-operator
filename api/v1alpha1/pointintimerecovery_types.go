package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PointInTimeRecoverySpec defines the desired state of PointInTimeRecovery.
type PointInTimeRecoverySpec struct {
	// Foo is an example field of PointInTimeRecovery. Edit pointintimerecovery_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// PointInTimeRecoveryStatus defines the observed state of PointInTimeRecovery.
type PointInTimeRecoveryStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PointInTimeRecovery is the Schema for the pointintimerecoveries API.
type PointInTimeRecovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PointInTimeRecoverySpec   `json:"spec,omitempty"`
	Status PointInTimeRecoveryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PointInTimeRecoveryList contains a list of PointInTimeRecovery.
type PointInTimeRecoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PointInTimeRecovery `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PointInTimeRecovery{}, &PointInTimeRecoveryList{})
}
