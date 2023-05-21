package galera

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GaleraReconciler struct {
	client.Client
	Builder             *builder.Builder
	RefResolver         *refresolver.RefResolver
	ConfigMapReconciler *configmap.ConfigMapReconciler
	ServiceReconciler   *service.ServiceReconciler
}

func NewGaleraReconciler(client client.Client, builder *builder.Builder, configMapReconciler *configmap.ConfigMapReconciler,
	serviceReconciler *service.ServiceReconciler) *GaleraReconciler {
	return &GaleraReconciler{
		Client:              client,
		Builder:             builder,
		RefResolver:         refresolver.New(client),
		ConfigMapReconciler: configMapReconciler,
		ServiceReconciler:   serviceReconciler,
	}
}

func (r *GaleraReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Galera == nil || mariadb.IsRestoringBackup() {
		return nil
	}
	if err := r.SetConfiguringGalera(ctx, mariadb); err != nil {
		return err
	}

	healthy, err := health.IsMariaDBHealthy(ctx, r.Client, mariadb, health.EndpointPolicyAll)
	if err != nil {
		return err
	}
	if !healthy {
		return nil
	}
	mdbClient, err := mariadbclient.NewRootClient(ctx, mariadb, r.RefResolver)
	if err != nil {
		return err
	}
	defer mdbClient.Close()

	return r.SetConfiguredGalera(ctx, mariadb, mdbClient)
}

func (r *GaleraReconciler) SetConfiguringGalera(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if meta.FindStatusCondition(mariadb.Status.Conditions, mariadbv1alpha1.ConditionTypeGaleraConfigured) != nil {
		return nil
	}
	return r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetConfiguringGalera(&mariadb.Status, mariadb)
	})
}

func (r *GaleraReconciler) SetConfiguredGalera(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	mdbClient *mariadbclient.Client) error {
	clusterSize, err := mdbClient.GaleraClusterSize(ctx)
	if err != nil {
		return err
	}

	if clusterSize != int(mariadb.Spec.Replicas) {
		return nil
	}
	return r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetConfiguredGalera(&mariadb.Status)
	})
}

func (r *GaleraReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
