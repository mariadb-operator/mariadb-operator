package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildServiceAccount(key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB) (*corev1.ServiceAccount, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	sa := &corev1.ServiceAccount{
		ObjectMeta: objMeta,
	}
	if err := controllerutil.SetControllerReference(mariadb, sa, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ServiceAccount: %v", err)
	}
	return sa, nil
}

func (b *Builder) BuildRole(key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB, rules []rbacv1.PolicyRule) (*rbacv1.Role, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	r := &rbacv1.Role{
		ObjectMeta: objMeta,
		Rules:      rules,
	}
	if err := controllerutil.SetControllerReference(mariadb, r, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Role: %v", err)
	}
	return r, nil
}

func (b *Builder) BuildRoleBinding(key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB, r *rbacv1.Role,
	sa *corev1.ServiceAccount) (*rbacv1.RoleBinding, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	rb := &rbacv1.RoleBinding{
		ObjectMeta: objMeta,
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     r.Name,
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, rb, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to RoleBinding: %v", err)
	}
	return rb, nil
}
