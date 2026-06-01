// Package v1alpha1 contains API Schema definitions for the v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=k8s.mariadb.com
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// BackupKind is the kind name of Backup
	BackupKind = "Backup"
	// PhysicalBackupKind is the kind name of PhysicalBackup
	PhysicalBackupKind = "PhysicalBackup"
	// ExternalMariaDBKind is the kind name of ExternalMariaDB
	ExternalMariaDBKind = "ExternalMariaDB"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "k8s.mariadb.com", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&Backup{}, &BackupList{},
		&Connection{}, &ConnectionList{},
		&Database{}, &DatabaseList{},
		&ExternalMariaDB{}, &ExternalMariaDBList{},
		&Grant{}, &GrantList{},
		&MariaDB{}, &MariaDBList{},
		&MaxScale{}, &MaxScaleList{},
		&PhysicalBackup{}, &PhysicalBackupList{},
		&PointInTimeRecovery{}, &PointInTimeRecoveryList{},
		&Restore{}, &RestoreList{},
		&SqlJob{}, &SqlJobList{},
		&User{}, &UserList{},
	)

	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
