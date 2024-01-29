package replication

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
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
	opts ...Option) (*ReplicationReconciler, error) {
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
		reconciler, err := secret.NewSecretReconciler(client, builder)
		if err != nil {
			return nil, err
		}
		r.secretReconciler = reconciler
	}
	if r.serviceReconciler == nil {
		r.serviceReconciler = service.NewServiceReconciler(client)
	}
	return r, nil
}

type reconcileRequest struct {
	mariadb   *mariadbv1alpha1.MariaDB
	key       types.NamespacedName
	clientSet *ReplicationClientSet
}

type replicationPhase struct {
	name      string
	key       types.NamespacedName
	reconcile func(context.Context, *reconcileRequest, logr.Logger) error
}

func (r *ReplicationReconciler) Reconcile(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
	if !mdb.Replication().Enabled || mdb.IsRestoringBackup() {
		return nil
	}
	logger := replLogger(ctx)

	if !mdb.IsMaxScaleEnabled() && mdb.IsSwitchingPrimary() {
		clientSet, err := NewReplicationClientSet(mdb, r.refResolver)
		if err != nil {
			return fmt.Errorf("error creating mariadb clientset: %v", err)
		}
		defer clientSet.close()

		req := reconcileRequest{
			mariadb:   mdb,
			key:       client.ObjectKeyFromObject(mdb),
			clientSet: clientSet,
		}
		if err := r.reconcileSwitchover(ctx, &req, logger.WithName("switchover")); err != nil {
			return fmt.Errorf("error recovering primary switchover: %v", err)
		}
		return nil
	}
	healthy, err := health.IsMariaDBHealthy(ctx, r.Client, mdb, health.EndpointPolicyAll)
	if err != nil {
		return fmt.Errorf("error checking MariaDB health: %v", err)
	}
	if !healthy {
		return nil
	}

	clientSet, err := NewReplicationClientSet(mdb, r.refResolver)
	if err != nil {
		return fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	defer clientSet.close()

	mariaDbKey := client.ObjectKeyFromObject(mdb)
	phases := []replicationPhase{
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
			name:      "reconcile switchover",
			key:       mariaDbKey,
			reconcile: r.reconcileSwitchover,
		},
	}

	for _, p := range phases {
		req := reconcileRequest{
			mariadb:   mdb,
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

func (r *ReplicationReconciler) reconcilePrimary(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	if req.mariadb.IsReplicationConfigured() || req.mariadb.IsSwitchingPrimary() {
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
	if req.mariadb.IsReplicationConfigured() || req.mariadb.IsSwitchingPrimary() {
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
		if err := r.replConfig.ConfigureReplica(ctx, req.mariadb, client, i, *req.mariadb.Replication().Primary.PodIndex, false); err != nil {
			return fmt.Errorf("error configuring replica '%d': %v", i, err)
		}
	}
	return nil
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}

func replLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("replication")
}
