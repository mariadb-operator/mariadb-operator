package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type UserOpts struct {
	Name                 string
	Host                 string
	PasswordSecretKeyRef *v1.SecretKeySelector
	MaxUserConnections   int32
	Metadata             *mariadbv1alpha1.Metadata
	MariaDBRef           mariadbv1alpha1.MariaDBRef
}

func (b *Builder) BuildUser(key types.NamespacedName, owner metav1.Object, opts UserOpts) (*mariadbv1alpha1.User, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(opts.Metadata).
			Build()
	user := &mariadbv1alpha1.User{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.UserSpec{
			MariaDBRef:           opts.MariaDBRef,
			Name:                 opts.Name,
			Host:                 opts.Host,
			PasswordSecretKeyRef: opts.PasswordSecretKeyRef,
		},
	}
	if opts.MaxUserConnections > 0 {
		user.Spec.MaxUserConnections = opts.MaxUserConnections
	}
	if err := controllerutil.SetControllerReference(owner, user, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to User: %v", err)
	}
	return user, nil
}

type GrantOpts struct {
	Privileges  []string
	Database    string
	Table       string
	Username    string
	Host        string
	GrantOption bool
	Metadata    *mariadbv1alpha1.Metadata
	MariaDBRef  mariadbv1alpha1.MariaDBRef
}

func (b *Builder) BuildGrant(key types.NamespacedName, owner metav1.Object, opts GrantOpts) (*mariadbv1alpha1.Grant, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(opts.Metadata).
			Build()
	grant := &mariadbv1alpha1.Grant{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.GrantSpec{
			MariaDBRef:  opts.MariaDBRef,
			Privileges:  opts.Privileges,
			Database:    opts.Database,
			Table:       opts.Table,
			Username:    opts.Username,
			GrantOption: opts.GrantOption,
		},
	}
	if opts.Host != "" {
		grant.Spec.Host = &opts.Host
	}
	if err := controllerutil.SetControllerReference(owner, grant, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Grant: %v", err)
	}
	return grant, nil
}

type DatabaseOpts struct {
	Name       string
	Metadata   *mariadbv1alpha1.Metadata
	MariaDBRef mariadbv1alpha1.MariaDBRef
}

func (b *Builder) BuildDatabase(key types.NamespacedName, owner metav1.Object, opts DatabaseOpts) (*mariadbv1alpha1.Database, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(opts.Metadata).
			Build()
	database := &mariadbv1alpha1.Database{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.DatabaseSpec{
			MariaDBRef: opts.MariaDBRef,
			Name:       opts.Name,
		},
	}
	if err := controllerutil.SetControllerReference(owner, database, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Database: %v", err)
	}
	return database, nil
}
