package replication

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	replresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/replication/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReplicationReconciler struct {
	client.Client
	ReplConfig       *ReplicationConfig
	SecretReconciler *secret.SecretReconciler
	Builder          *builder.Builder
	RefResolver      *refresolver.RefResolver
}

func NewReplicationReconciler(client client.Client, replConfig *ReplicationConfig,
	secretReconciler *secret.SecretReconciler, builder *builder.Builder) *ReplicationReconciler {
	return &ReplicationReconciler{
		Client:           client,
		ReplConfig:       replConfig,
		SecretReconciler: secretReconciler,
		Builder:          builder,
		RefResolver:      refresolver.New(client),
	}
}

type reconcileRequest struct {
	mariadb   *mariadbv1alpha1.MariaDB
	key       types.NamespacedName
	clientSet *mariadbClientSet
}

type replicationPhase struct {
	name      string
	key       types.NamespacedName
	reconcile func(context.Context, *reconcileRequest) error
}

func (r *ReplicationReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Replication == nil {
		return nil
	}
	if mariadb.IsSwitchingPrimary() {
		clientSet, err := newMariaDBClientSet(mariadb, r.RefResolver)
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

	clientSet, err := newMariaDBClientSet(mariadb, r.RefResolver)
	if err != nil {
		return fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	defer clientSet.close()

	mariaDbKey := client.ObjectKeyFromObject(mariadb)
	phases := []replicationPhase{
		{
			name:      "set configure replication status",
			key:       mariaDbKey,
			reconcile: r.setConfigureReplicationStatus,
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
			name:      "reconcile PodDisruptionBudget",
			key:       replresources.PodDisruptionBudgetKey(mariadb),
			reconcile: r.reconcilePodDisruptionBudget,
		},
		{
			name:      "reconcile primary Service",
			reconcile: r.reconcilePrimaryService,
			key:       replresources.PrimaryServiceKey(mariadb),
		},
		{
			name:      "reconcile primary Connection",
			key:       replresources.PrimaryConnectioneKey(mariadb),
			reconcile: r.reconcilePrimaryConn,
		},
		{
			name:      "update currentPrimaryPodIndex",
			key:       mariaDbKey,
			reconcile: r.updateCurrentPrimaryPodIndex,
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

func (r *ReplicationReconciler) setConfigureReplicationStatus(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	return r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		return conditions.SetConfiguringReplication(&req.mariadb.Status, req.mariadb)
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

func (r *ReplicationReconciler) reconcilePodDisruptionBudget(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Spec.PodDisruptionBudget != nil {
		return nil
	}
	key := replresources.PodDisruptionBudgetKey(req.mariadb)
	var existingPDB policyv1.PodDisruptionBudget
	if err := r.Get(ctx, key, &existingPDB); err == nil {
		return nil
	}

	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(req.mariadb).
			Build()
	minAvailable := intstr.FromString("50%")
	opts := builder.PodDisruptionBudgetOpts{
		MariaDB:        req.mariadb,
		Key:            key,
		MinAvailable:   &minAvailable,
		SelectorLabels: selectorLabels,
	}
	pdb, err := r.Builder.BuildPodDisruptionBudget(&opts, req.mariadb)
	if err != nil {
		return fmt.Errorf("error building PodDisruptionBudget: %v", err)
	}

	return r.Create(ctx, pdb)
}

func (r *ReplicationReconciler) reconcilePrimaryService(ctx context.Context, req *reconcileRequest) error {
	serviceLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(req.mariadb).
			WithStatefulSetPod(req.mariadb, req.mariadb.Spec.Replication.Primary.PodIndex).
			Build()
	opts := builder.ServiceOpts{
		Selectorlabels: serviceLabels,
	}
	if req.mariadb.Spec.Replication.Primary.Service != nil {
		opts.Type = req.mariadb.Spec.Replication.Primary.Service.Type
		opts.Annotations = req.mariadb.Spec.Replication.Primary.Service.Annotations
	}
	desiredSvc, err := r.Builder.BuildService(req.mariadb, req.key, opts)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}

	var existingSvc corev1.Service
	if err := r.Get(ctx, req.key, &existingSvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting Service: %v", err)
		}
		if err := r.Create(ctx, desiredSvc); err != nil {
			return err
		}
		return nil
	}

	patch := client.MergeFrom(existingSvc.DeepCopy())
	existingSvc.Spec.Ports = desiredSvc.Spec.Ports

	return r.Patch(ctx, &existingSvc, patch)
}

func (r *ReplicationReconciler) reconcilePrimaryConn(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Spec.Replication.Primary.Connection == nil ||
		req.mariadb.Spec.Username == nil || req.mariadb.Spec.PasswordSecretKeyRef == nil ||
		!req.mariadb.IsReady() {
		return nil
	}
	var existingConn mariadbv1alpha1.Connection
	if err := r.Get(ctx, req.key, &existingConn); err == nil {
		return nil
	}

	connTpl := req.mariadb.Spec.Replication.Primary.Connection
	if req.mariadb.Spec.Replication != nil {
		serviceName := replresources.PrimaryServiceKey(req.mariadb).Name
		connTpl.ServiceName = &serviceName
	}

	connOpts := builder.ConnectionOpts{
		MariaDB:              req.mariadb,
		Key:                  req.key,
		Username:             *req.mariadb.Spec.Username,
		PasswordSecretKeyRef: *req.mariadb.Spec.PasswordSecretKeyRef,
		Database:             req.mariadb.Spec.Database,
		Template:             connTpl,
	}
	conn, err := r.Builder.BuildConnection(connOpts, req.mariadb)
	if err != nil {
		return fmt.Errorf("erro building primary Connection: %v", err)
	}

	return r.Create(ctx, conn)
}

func (r *ReplicationReconciler) updateCurrentPrimaryPodIndex(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	return r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.UpdateCurrentPrimary(req.mariadb, req.mariadb.Spec.Replication.Primary.PodIndex)
		return nil
	})
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus) error) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	if err := patcher(&mariadb.Status); err != nil {
		return err
	}

	return r.Status().Patch(ctx, mariadb, patch)
}

func replPasswordKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("repl-password-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
