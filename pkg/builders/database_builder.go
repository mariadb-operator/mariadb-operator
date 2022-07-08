package builders

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UserOpts struct {
	Name                 string
	PasswordSecretKeyRef v1.SecretKeySelector
	MaxUserConnections   int32
}

func BuildUser(mariadb *databasev1alpha1.MariaDB, opts UserOpts) *databasev1alpha1.UserMariaDB {
	labels := NewLabelsBuilder().WithObjectMeta(mariadb.ObjectMeta).WithApp(app).Build()
	return &databasev1alpha1.UserMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: mariadb.Namespace,
			Labels:    labels,
		},
		Spec: databasev1alpha1.UserMariaDBSpec{
			MariaDBRef: corev1.LocalObjectReference{
				Name: mariadb.Name,
			},
			PasswordSecretKeyRef: opts.PasswordSecretKeyRef,
			MaxUserConnections:   opts.MaxUserConnections,
		},
	}
}

type GrantOpts struct {
	Name        string
	Privileges  []string
	Database    string
	Table       string
	Username    string
	GrantOption bool
}

func BuildGrant(mariadb *databasev1alpha1.MariaDB, opts GrantOpts) *databasev1alpha1.GrantMariaDB {
	labels := NewLabelsBuilder().WithObjectMeta(mariadb.ObjectMeta).WithApp(app).Build()
	return &databasev1alpha1.GrantMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: mariadb.Namespace,
			Labels:    labels,
		},
		Spec: databasev1alpha1.GrantMariaDBSpec{
			MariaDBRef: corev1.LocalObjectReference{
				Name: mariadb.Name,
			},
			Privileges:  opts.Privileges,
			Database:    opts.Database,
			Table:       opts.Table,
			Username:    opts.Username,
			GrantOption: opts.GrantOption,
		},
	}
}
