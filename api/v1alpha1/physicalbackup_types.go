package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PhysicalBackupSpec defines the desired state of PhysicalBackup.
type PhysicalBackupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of PhysicalBackup. Edit physicalbackup_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// PhysicalBackupStatus defines the observed state of PhysicalBackup.
type PhysicalBackupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PhysicalBackup is the Schema for the physicalbackups API.
type PhysicalBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PhysicalBackupSpec   `json:"spec,omitempty"`
	Status PhysicalBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PhysicalBackupList contains a list of PhysicalBackup.
type PhysicalBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PhysicalBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PhysicalBackup{}, &PhysicalBackupList{})
}
