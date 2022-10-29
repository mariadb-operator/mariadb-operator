package builder

import (
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	labels "github.com/mmontes11/mariadb-operator/pkg/builder/labels"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type UserOpts struct {
	Key                  types.NamespacedName
	PasswordSecretKeyRef v1.SecretKeySelector
	MaxUserConnections   int32
}

func (b *Builder) BuildUserMariaDB(mariadb *databasev1alpha1.MariaDB, opts UserOpts) (*databasev1alpha1.UserMariaDB, error) {
	databaseLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			Build()
	user := &databasev1alpha1.UserMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Key.Name,
			Namespace: opts.Key.Namespace,
			Labels:    databaseLabels,
		},
		Spec: databasev1alpha1.UserMariaDBSpec{
			MariaDBRef: databasev1alpha1.MariaDBRef{
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
		return nil, fmt.Errorf("error setting controller reference to UserMariaDB: %v", err)
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

func (b *Builder) BuildGrantMariaDB(mariadb *databasev1alpha1.MariaDB, opts GrantOpts) (*databasev1alpha1.GrantMariaDB, error) {
	grantLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			Build()
	grant := &databasev1alpha1.GrantMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Key.Name,
			Namespace: opts.Key.Namespace,
			Labels:    grantLabels,
		},
		Spec: databasev1alpha1.GrantMariaDBSpec{
			MariaDBRef: databasev1alpha1.MariaDBRef{
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
		return nil, fmt.Errorf("error setting controller reference to GrantMariaDB: %v", err)
	}

	return grant, nil
}
