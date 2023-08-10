package replication

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Option func(*ReplicationReconciler)

func WithRefResolver(rr *refresolver.RefResolver) Option {
	return func(r *ReplicationReconciler) {
		r.refResolver = rr
	}
}

func WithSecretReconciler(sr *secret.SecretReconciler) Option {
	return func(rr *ReplicationReconciler) {
		rr.secretReconciler = sr
	}
}

func WithServiceReconciler(sr *service.ServiceReconciler) Option {
	return func(rr *ReplicationReconciler) {
		rr.serviceReconciler = sr
	}
}

type ReplicationReconciler struct {
	client.Client
	recorder          record.EventRecorder
	builder           *builder.Builder
	replConfig        *ReplicationConfig
	refResolver       *refresolver.RefResolver
	secretReconciler  *secret.SecretReconciler
	serviceReconciler *service.ServiceReconciler
}

func NewReplicationReconciler(client client.Client, recorder record.EventRecorder, builder *builder.Builder, replConfig *ReplicationConfig,
	opts ...Option) *ReplicationReconciler {
	r := &ReplicationReconciler{
		Client:     client,
		recorder:   recorder,
		builder:    builder,
		replConfig: replConfig,
	}
	for _, setOpt := range opts {
		setOpt(r)
	}
	if r.refResolver == nil {
		r.refResolver = refresolver.New(client)
	}
	if r.secretReconciler == nil {
		r.secretReconciler = secret.NewSecretReconciler(client, builder)
	}
	if r.serviceReconciler == nil {
		r.serviceReconciler = service.NewServiceReconciler(client)
	}
	return r
}

type reconcileRequest struct {
	mariadb   *mariadbv1alpha1.MariaDB
	key       types.NamespacedName
	clientSet *replicationClientSet
}

type replicationPhase struct {
	name      string
	key       types.NamespacedName
	reconcile func(context.Context, *reconcileRequest, logr.Logger) error
}

func (r *ReplicationReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if !mariadb.Replication().Enabled || mariadb.IsRestoringBackup() {
		return nil
	}
	logger := log.FromContext(ctx).WithName("replication")

	if mariadb.IsSwitchingPrimary() {
		clientSet, err := newReplicationClientSet(mariadb, r.refResolver)
		if err != nil {
			return fmt.Errorf("error creating mariadb clientset: %v", err)
		}
		defer clientSet.close()

		req := reconcileRequest{
			mariadb:   mariadb,
			key:       client.ObjectKeyFromObject(mariadb),
			clientSet: clientSet,
		}
		if err := r.reconcileSwitchover(ctx, &req, logger.WithName("switchover")); err != nil {
			return fmt.Errorf("error recovering primary switchover: %v", err)
		}
		return nil
	}
	healthy, err := health.IsMariaDBHealthy(ctx, r.Client, mariadb, health.EndpointPolicyAll)
	if err != nil {
		return fmt.Errorf("error checking MariaDB health: %v", err)
	}
	if !healthy {
		return nil
	}

	clientSet, err := newReplicationClientSet(mariadb, r.refResolver)
	if err != nil {
		return fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	defer clientSet.close()

	mariaDbKey := client.ObjectKeyFromObject(mariadb)
	phases := []replicationPhase{
		{
			name:      "set configuring replication status",
			key:       mariaDbKey,
			reconcile: r.setConfiguringReplication,
		},
		{
			name:      "reconcile Primary",
			key:       mariaDbKey,
			reconcile: r.reconcilePrimary,
		},
		{
			name:      "reconcile Replicas",
			key:       mariaDbKey,
			reconcile: r.reconcileReplicas,
		},
		{
			name:      "set configured replication status",
			key:       mariaDbKey,
			reconcile: r.setConfiguredReplication,
		},
		{
			name:      "reconcile switchover",
			key:       mariaDbKey,
			reconcile: r.reconcileSwitchover,
		},
	}

	for _, p := range phases {
		req := reconcileRequest{
			mariadb:   mariadb,
			key:       p.key,
			clientSet: clientSet,
		}
		if err := p.reconcile(ctx, &req, logger); err != nil {
			if apierrors.IsNotFound(err) {
				return err
			}
			return fmt.Errorf("error reconciling '%s' phase: %v", p.name, err)
		}
	}
	return nil
}

func (r *ReplicationReconciler) setConfiguringReplication(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	logger.Info("Configuring replication")
	r.recorder.Event(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationConfiguring, "Configuring replication")

	return r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetConfiguringReplication(&req.mariadb.Status, req.mariadb)
	})
}

func (r *ReplicationReconciler) reconcilePrimary(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	client, err := req.clientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	podIndex := *req.mariadb.Replication().Primary.PodIndex
	logger.V(1).Info("Configuring primary", "pod-index", podIndex)
	return r.replConfig.ConfigurePrimary(ctx, req.mariadb, client, podIndex)
}

func (r *ReplicationReconciler) reconcileReplicas(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	logger.V(1).Info("Configuring replicas")
	for i := 0; i < int(req.mariadb.Spec.Replicas); i++ {
		if i == *req.mariadb.Replication().Primary.PodIndex {
			continue
		}
		client, err := req.clientSet.clientForIndex(ctx, i)
		if err != nil {
			return fmt.Errorf("error getting client for replica '%d': %v", i, err)
		}

		logger.V(1).Info("Configuring replica", "pod-index", i)
		if err := r.replConfig.ConfigureReplica(ctx, req.mariadb, client, i, *req.mariadb.Replication().Primary.PodIndex); err != nil {
			return fmt.Errorf("error configuring replica '%d': %v", i, err)
		}
	}
	return nil
}

func (r *ReplicationReconciler) setConfiguredReplication(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	logger.Info("Replication configured")
	r.recorder.Event(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationConfigured, "Replication configured")

	return r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		status.UpdateCurrentPrimary(req.mariadb, *req.mariadb.Replication().Primary.PodIndex)
		conditions.SetConfiguredReplication(&req.mariadb.Status, req.mariadb)
	})
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
