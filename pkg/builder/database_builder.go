package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type UserOpts struct {
	Key                  types.NamespacedName
	PasswordSecretKeyRef v1.SecretKeySelector
	MaxUserConnections   int32
}

func (b *Builder) BuildUser(mariadb *mariadbv1alpha1.MariaDB, opts UserOpts) (*mariadbv1alpha1.User, error) {
	objMeta :=
		metadata.NewMetadataBuilder(opts.Key).
			WithMariaDB(mariadb).
			Build()
	user := &mariadbv1alpha1.User{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.UserSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mariadb.Name,
				},
				WaitForIt: true,
			},
			PasswordSecretKeyRef: opts.PasswordSecretKeyRef,
			MaxUserConnections:   opts.MaxUserConnections,
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, user, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to User: %v", err)
	}

	return user, nil
}

type GrantOpts struct {
	Key         types.NamespacedName
	Privileges  []string
	Database    string
	Table       string
	Username    string
	GrantOption bool
}

func (b *Builder) BuildGrant(mariadb *mariadbv1alpha1.MariaDB, opts GrantOpts) (*mariadbv1alpha1.Grant, error) {
	objMeta :=
		metadata.NewMetadataBuilder(opts.Key).
			WithMariaDB(mariadb).
			Build()
	grant := &mariadbv1alpha1.Grant{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.GrantSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mariadb.Name,
				},
				WaitForIt: true,
			},
			Privileges:  opts.Privileges,
			Database:    opts.Database,
			Table:       opts.Table,
			Username:    opts.Username,
			GrantOption: opts.GrantOption,
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, grant, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Grant: %v", err)
	}

	return grant, nil
}
