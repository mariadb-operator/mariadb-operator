package rbac

import (
	"context"
	"slices"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileMariadbRBACGrantsInitContainerReadAccess(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding rbac scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb).
		Build()
	disc, err := discovery.NewFakeDiscovery()
	if err != nil {
		t.Fatalf("error creating fake discovery: %v", err)
	}
	reconciler := NewRBACReconciler(fakeClient, builder.NewBuilder(scheme, &environment.OperatorEnv{}, disc))

	if err := reconciler.ReconcileMariadbRBAC(context.Background(), mariadb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var role rbacv1.Role
	roleKey := client.ObjectKey{
		Name:      mariadb.Spec.ServiceAccountKey(mariadb.ObjectMeta).Name,
		Namespace: mariadb.Namespace,
	}
	if err := fakeClient.Get(context.Background(), roleKey, &role); err != nil {
		t.Fatalf("error getting Role: %v", err)
	}

	tests := []struct {
		name     string
		apiGroup string
		resource string
		verb     string
	}{
		{
			name:     "mariadbs get",
			apiGroup: mariadbv1alpha1.GroupVersion.Group,
			resource: "mariadbs",
			verb:     "get",
		},
		{
			name:     "jobs get",
			apiGroup: batchv1.GroupName,
			resource: "jobs",
			verb:     "get",
		},
		{
			name:     "persistentvolumeclaims get",
			apiGroup: corev1.GroupName,
			resource: "persistentvolumeclaims",
			verb:     "get",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !roleAllows(role.Rules, tt.apiGroup, tt.resource, tt.verb) {
				t.Fatalf("expected Role to allow %s %s in group %q", tt.verb, tt.resource, tt.apiGroup)
			}
		})
	}
}

func roleAllows(rules []rbacv1.PolicyRule, apiGroup, resource, verb string) bool {
	for _, rule := range rules {
		if slices.Contains(rule.APIGroups, apiGroup) &&
			slices.Contains(rule.Resources, resource) &&
			slices.Contains(rule.Verbs, verb) {
			return true
		}
	}
	return false
}
