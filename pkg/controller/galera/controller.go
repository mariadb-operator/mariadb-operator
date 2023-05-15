package galera

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GaleraReconciler struct {
	client.Client
	RefResolver *refresolver.RefResolver
}

func NewGaleraReconciler(client client.Client) *GaleraReconciler {
	return &GaleraReconciler{
		Client:      client,
		RefResolver: refresolver.New(client),
	}
}

func (r *GaleraReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Galera == nil || mariadb.IsRestoringBackup() {
		return nil
	}
	healthy, err := health.IsMariaDBHealthy(ctx, r.Client, mariadb, health.EndpointPolicyAll)
	if err != nil {
		return fmt.Errorf("error checking MariaDB health: %v", err)
	}
	if !healthy {
		return nil
	}

	if err := r.reconcileConfigMap(ctx, mariadb); err != nil {
		return fmt.Errorf("error reconciling galera ConfigMap: %v", err)
	}
	return nil
}

func (r *GaleraReconciler) reconcileConfigMap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	return nil
}
