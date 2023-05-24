package galera

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	if err := r.InitGalera(ctx, mariadb); err != nil {
		return err
	}
	sts, err := r.statefulSet(ctx, mariadb)
	if err != nil {
		return err
	}

	if sts.Status.ReadyReplicas == 0 {
		log.FromContext(ctx).V(1).Info("Recovering Galera cluster")
		return r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			conditions.SetGaleraNotReady(&mariadb.Status, mariadb)
		})
	}
	if mariadb.IsGaleraNotReady() && sts.Status.ReadyReplicas == mariadb.Spec.Replicas {
		log.FromContext(ctx).V(1).Info("Disabling Galera bootstrap")
		return r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			conditions.SetGaleraReady(&mariadb.Status)
		})
	}
	return nil
}

func (r *GaleraReconciler) InitGalera(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if meta.FindStatusCondition(mariadb.Status.Conditions, mariadbv1alpha1.ConditionTypeGaleraReady) != nil {
		return nil
	}
	return r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetGaleraNotReady(&mariadb.Status, mariadb)
	})
}

func (r *GaleraReconciler) statefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*appsv1.StatefulSet, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		return nil, err
	}
	return &sts, nil
}

func (r *GaleraReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
