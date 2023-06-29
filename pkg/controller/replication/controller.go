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
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	corev1 "k8s.io/api/core/v1"
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

func (r *ReplicationReconciler) reconcilePrimaryService(ctx context.Context, req *reconcileRequest) error {
	serviceLabels :=
		labels.NewLabelsBuilder().
			WithMariaDBSelectorLabels(req.mariadb).
			WithStatefulSetPod(req.mariadb, req.mariadb.Spec.Replication.Primary.PodIndex).
			Build()
	opts := builder.ServiceOpts{
		Selectorlabels: serviceLabels,
		Ports: []corev1.ServicePort{
			{
				Name: builder.MariaDbContainerName,
				Port: req.mariadb.Spec.Port,
			},
		},
	}
	if req.mariadb.Spec.Replication.Primary.Service != nil {
		opts.Type = req.mariadb.Spec.Replication.Primary.Service.Type
		opts.Annotations = req.mariadb.Spec.Replication.Primary.Service.Annotations
	}
	desiredSvc, err := r.Builder.BuildService(req.mariadb, req.key, opts)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
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

func replPasswordKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("repl-password-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
