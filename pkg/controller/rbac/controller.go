package rbac

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RBACReconciler struct {
	client.Client
	builder *builder.Builder
}

func NewRBACReconiler(client client.Client, builder *builder.Builder) *RBACReconciler {
	return &RBACReconciler{
		Client:  client,
		builder: builder,
	}
}

func (r *RBACReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Galera == nil {
		return nil
	}
	sa, err := r.reconcileServiceAccount(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error reconciling ServiceAccount: %v", err)
	}

	role, err := r.reconcileRole(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error reconciling Role: %v", err)
	}

	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}
	if err := r.reconcileRoleBinding(ctx, client.ObjectKeyFromObject(mariadb), mariadb, sa, roleRef); err != nil {
		return fmt.Errorf("error reconciling RoleBinding: %v", err)
	}

	if mariadb.Spec.Galera.Agent.KubernetesAuth.Enabled {
		authDelegatorRoleRef := rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:auth-delegator",
		}
		key := types.NamespacedName{
			Name:      fmt.Sprintf("%s:auth-delegator", mariadb.Spec.Galera.Agent.KubernetesAuth.AuthDelegatorRoleNameOrDefault(mariadb)),
			Namespace: mariadb.Namespace,
		}
		if err := r.reconcileClusterRoleBinding(ctx, key, mariadb, sa, authDelegatorRoleRef); err != nil {
			return fmt.Errorf("error reconciling system:auth-delegator ClusterRoleBinding: %v", err)
		}
	}
	return nil
}

func (r *RBACReconciler) reconcileServiceAccount(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*corev1.ServiceAccount, error) {
	key := client.ObjectKeyFromObject(mariadb)
	var existingSA corev1.ServiceAccount
	err := r.Get(ctx, key, &existingSA)
	if err == nil {
		return &existingSA, nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("error getting ServiceAccount: %v", err)
	}

	sa, err := r.builder.BuildServiceAccount(key, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error building ServiceAccount: %v", err)
	}
	if err := r.Create(ctx, sa); err != nil {
		return nil, fmt.Errorf("error creating ServiceAccount: %v", err)
	}
	return sa, nil
}

func (r *RBACReconciler) reconcileRole(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*rbacv1.Role, error) {
	key := client.ObjectKeyFromObject(mariadb)
	var existingRole rbacv1.Role
	err := r.Get(ctx, key, &existingRole)
	if err == nil {
		return &existingRole, nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("error getting Role: %v", err)
	}

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				mariadbv1alpha1.GroupVersion.Group,
			},
			Resources: []string{
				"mariadbs",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				corev1.GroupName,
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"get",
			},
		},
	}
	role, err := r.builder.BuildRole(key, mariadb, rules)
	if err != nil {
		return nil, fmt.Errorf("error building Role: %v", err)
	}
	if err := r.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("error creating Role: %v", err)
	}
	return role, nil
}

func (r *RBACReconciler) reconcileRoleBinding(ctx context.Context, key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB,
	sa *corev1.ServiceAccount, roleRef rbacv1.RoleRef) error {
	var existingRB rbacv1.RoleBinding
	err := r.Get(ctx, key, &existingRB)
	if err == nil {
		return nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting RoleBinding: %v", err)
	}

	rb, err := r.builder.BuildRoleBinding(key, mariadb, sa, roleRef)
	if err != nil {
		return fmt.Errorf("error building RoleBinding: %v", err)
	}
	if err := r.Create(ctx, rb); err != nil {
		return fmt.Errorf("error creating RoleBinding: %v", err)
	}
	return nil
}

func (r *RBACReconciler) reconcileClusterRoleBinding(ctx context.Context, key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB,
	sa *corev1.ServiceAccount, roleRef rbacv1.RoleRef) error {
	var existingCRB rbacv1.ClusterRoleBinding
	err := r.Get(ctx, key, &existingCRB)
	if err == nil {
		return nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting ClusterRoleBinding: %v", err)
	}

	crdb, err := r.builder.BuildClusterRoleBinding(key, mariadb, sa, roleRef)
	if err != nil {
		return fmt.Errorf("error building ClusterRoleBinding: %v", err)
	}
	if err := r.Create(ctx, crdb); err != nil {
		return fmt.Errorf("error creating ClusterRoleBinding: %v", err)
	}
	return nil
}
