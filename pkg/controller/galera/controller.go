package galera

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	agentclient "github.com/mariadb-operator/agent/pkg/client"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Option func(*GaleraReconciler)

func WithRefResolver(rr *refresolver.RefResolver) Option {
	return func(r *GaleraReconciler) {
		r.refResolver = rr
	}
}

func WithConfigMapReconciler(cmr *configmap.ConfigMapReconciler) Option {
	return func(r *GaleraReconciler) {
		r.configMapReconciler = cmr
	}
}

func WithServiceReconciler(sr *service.ServiceReconciler) Option {
	return func(r *GaleraReconciler) {
		r.serviceReconciler = sr
	}
}

type GaleraReconciler struct {
	client.Client
	recorder            record.EventRecorder
	env                 *environment.Environment
	builder             *builder.Builder
	refResolver         *refresolver.RefResolver
	configMapReconciler *configmap.ConfigMapReconciler
	serviceReconciler   *service.ServiceReconciler
}

func NewGaleraReconciler(client client.Client, recorder record.EventRecorder, env *environment.Environment, builder *builder.Builder,
	opts ...Option) *GaleraReconciler {
	r := &GaleraReconciler{
		Client:   client,
		recorder: recorder,
		env:      env,
		builder:  builder,
	}
	for _, setOpt := range opts {
		setOpt(r)
	}
	if r.refResolver == nil {
		r.refResolver = refresolver.New(client)
	}
	if r.configMapReconciler == nil {
		r.configMapReconciler = configmap.NewConfigMapReconciler(client, builder)
	}
	if r.serviceReconciler == nil {
		r.serviceReconciler = service.NewServiceReconciler(client)
	}
	return r
}

func (r *GaleraReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if !mariadb.Galera().Enabled || mariadb.IsRestoringBackup() {
		return nil
	}
	sts, err := r.statefulSet(ctx, mariadb)
	if err != nil {
		return err
	}
	logger := log.FromContext(ctx).WithName("galera")

	if mariadb.HasGaleraNotReadyCondition() {
		if err := r.reconcileRecovery(ctx, mariadb, sts, logger.WithName("recovery")); err != nil {
			return err
		}
	}

	if !mariadb.HasGaleraReadyCondition() && sts.Status.ReadyReplicas == mariadb.Spec.Replicas {
		if err := r.disableBootstrap(ctx, mariadb, logger); err != nil {
			return err
		}
		logger.Info("Galera cluster is healthy")
		r.recorder.Event(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonGaleraClusterHealthy, "Galera cluster is healthy")

		return r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			conditions.SetGaleraReady(&mariadb.Status)
			conditions.SetGaleraConfigured(&mariadb.Status)
		})
	}
	return nil
}

func (r *GaleraReconciler) statefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*appsv1.StatefulSet, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		return nil, err
	}
	return &sts, nil
}

func (r *GaleraReconciler) disableBootstrap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	logger.V(1).Info("Disabling Galera bootstrap")

	clientSet, err := r.newAgentClientSet(mariadb)
	if err != nil {
		return fmt.Errorf("error creating agent client set: %v", err)
	}
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		agentClient, err := clientSet.clientForIndex(i)
		if err != nil {
			return fmt.Errorf("error creating agent client: %v", err)
		}
		if err := agentClient.Bootstrap.Disable(ctx); err != nil && !agentclient.IsNotFound(err) {
			return fmt.Errorf("error disabling bootstrap in Pod %d: %v", i, err)
		}
	}
	return nil
}

func (r *GaleraReconciler) newAgentClientSet(mariadb *mariadbv1alpha1.MariaDB, clientOpts ...agentclient.Option) (*agentClientSet, error) {
	opts := []agentclient.Option{}
	opts = append(opts, clientOpts...)
	if mariadb.Galera().Agent.KubernetesAuth.Enabled {
		opts = append(opts,
			agentclient.WithKubernetesAuth(true, r.env.MariadbOperatorSAPath),
		)
	}
	return newAgentClientSet(mariadb, opts...)
}

func (r *GaleraReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
