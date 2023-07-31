package replication

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReplicationReconciler struct {
	client.Client
	Builder           *builder.Builder
	RefResolver       *refresolver.RefResolver
	ReplConfig        *ReplicationConfig
	SecretReconciler  *secret.SecretReconciler
	ServiceReconciler *service.ServiceReconciler
}

func NewReplicationReconciler(client client.Client, builder *builder.Builder, replConfig *ReplicationConfig,
	secretReconciler *secret.SecretReconciler, serviceReconciler *service.ServiceReconciler) *ReplicationReconciler {
	return &ReplicationReconciler{
		Client:            client,
		Builder:           builder,
		RefResolver:       refresolver.New(client),
		ReplConfig:        replConfig,
		SecretReconciler:  secretReconciler,
		ServiceReconciler: serviceReconciler,
	}
}

type reconcileRequest struct {
	mariadb   *mariadbv1alpha1.MariaDB
	key       types.NamespacedName
	clientSet *replicationClientSet
}

type replicationPhase struct {
	name      string
	key       types.NamespacedName
	reconcile func(context.Context, *reconcileRequest) error
}

func (r *ReplicationReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Replication == nil || mariadb.IsRestoringBackup() {
		return nil
	}
	if mariadb.IsSwitchingPrimary() {
		clientSet, err := newReplicationClientSet(mariadb, r.RefResolver)
		if err != nil {
			return fmt.Errorf("error creating mariadb clientset: %v", err)
		}
		defer clientSet.close()

		req := reconcileRequest{
			mariadb:   mariadb,
			key:       client.ObjectKeyFromObject(mariadb),
			clientSet: clientSet,
		}
		if err := r.reconcileSwitchover(ctx, &req); err != nil {
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

	clientSet, err := newReplicationClientSet(mariadb, r.RefResolver)
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
		if err := p.reconcile(ctx, &req); err != nil {
			if apierrors.IsNotFound(err) {
				return err
			}
			return fmt.Errorf("error reconciling '%s' phase: %v", p.name, err)
		}
	}
	return nil
}

func (r *ReplicationReconciler) setConfiguringReplication(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	return r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetConfiguringReplication(&req.mariadb.Status, req.mariadb)
	})
}

func (r *ReplicationReconciler) reconcilePrimary(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	client, err := req.clientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}
	return r.ReplConfig.ConfigurePrimary(ctx, req.mariadb, client, req.mariadb.Spec.Replication.Primary.PodIndex)
}

func (r *ReplicationReconciler) reconcileReplicas(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	for i := 0; i < int(req.mariadb.Spec.Replicas); i++ {
		if i == req.mariadb.Spec.Replication.Primary.PodIndex {
			continue
		}
		client, err := req.clientSet.clientForIndex(ctx, i)
		if err != nil {
			return fmt.Errorf("error getting client for replica '%d': %v", i, err)
		}
		if err := r.ReplConfig.ConfigureReplica(ctx, req.mariadb, client, i, req.mariadb.Spec.Replication.Primary.PodIndex); err != nil {
			return fmt.Errorf("error configuring replica '%d': %v", i, err)
		}
	}
	return nil
}

func (r *ReplicationReconciler) setConfiguredReplication(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	return r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		status.UpdateCurrentPrimary(req.mariadb, req.mariadb.Spec.Replication.Primary.PodIndex)
		conditions.SetConfiguredReplication(&req.mariadb.Status, req.mariadb)
	})
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
