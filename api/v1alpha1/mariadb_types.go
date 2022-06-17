/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Image struct {
	Repository string            `json:"repository,omitempty"`
	Tag        string            `json:"tag,omitempty"`
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

type Storage struct {
	ClassName   string                              `json:"className,omitempty"`
	Size        resource.Quantity                   `json:"size,omitempty"`
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// MariaDBSpec defines the desired state of MariaDB
type MariaDBSpec struct {
	RootPasswordSecretKeyRef corev1.SecretKeySelector `json:"rootPasswordSecretKeyRef,omitempty"`

	Database             string                   `json:"database,omitempty"`
	Username             string                   `json:"username,omitempty"`
	PasswordSecretKeyRef corev1.SecretKeySelector `json:"password,omitempty"`

	Image            Image                         `json:"image,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	Port int32 `json:"port,omitempty"`

	Storage Storage `json:"storage,omitempty"`

	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	Environment map[string]string `json:"environment,omitempty"`
}

// MariaDBStatus defines the observed state of MariaDB
type MariaDBStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MariaDB is the Schema for the mariadbs API
type MariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MariaDBSpec   `json:"spec,omitempty"`
	Status MariaDBStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MariaDBList contains a list of MariaDB
type MariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MariaDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MariaDB{}, &MariaDBList{})
}
