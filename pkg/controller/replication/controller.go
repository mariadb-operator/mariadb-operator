package replication

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
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
	recorder            record.EventRecorder
	builder             *builder.Builder
	replConfig          *ReplicationConfig
	refResolver         *refresolver.RefResolver
	secretReconciler    *secret.SecretReconciler
	configMapreconciler *configmap.ConfigMapReconciler
	serviceReconciler   *service.ServiceReconciler
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
	if r.configMapreconciler == nil {
		r.configMapreconciler = configmap.NewConfigMapReconciler(client, builder)
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

func (r *ReplicationReconciler) Reconcile(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mdb.Replication().Enabled {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).WithName("replication")
	switchoverLogger := log.FromContext(ctx).WithName("switchover")

	if !mdb.IsMaxScaleEnabled() && mdb.IsSwitchoverRequired() {
		clientSet, err := NewReplicationClientSet(mdb, r.refResolver)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating mariadb clientset: %v", err)
		}
		defer clientSet.close()

		req := reconcileRequest{
			mariadb:   mdb,
			key:       client.ObjectKeyFromObject(mdb),
			clientSet: clientSet,
		}
		return ctrl.Result{}, r.reconcileSwitchover(ctx, &req, switchoverLogger)
	}

	clientSet, err := NewReplicationClientSet(mdb, r.refResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	defer clientSet.close()

	req := reconcileRequest{
		mariadb:   mdb,
		key:       client.ObjectKeyFromObject(mdb),
		clientSet: clientSet,
	}
	if result, err := r.reconcileReplication(ctx, &req, logger); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, r.reconcileSwitchover(ctx, &req, switchoverLogger)
}

// nolint:lll
func (r *ReplicationReconciler) ReconcileProbeConfigMap(ctx context.Context, configMapKeyRef mariadbv1alpha1.ConfigMapKeySelector,
	mdb *mariadbv1alpha1.MariaDB) error {
	if !mdb.Replication().Enabled {
		return nil
	}
	req := configmap.ReconcileRequest{
		Metadata: mdb.Spec.InheritMetadata,
		Owner:    mdb,
		Key: types.NamespacedName{
			Name:      configMapKeyRef.Name,
			Namespace: mdb.Namespace,
		},
		Data: map[string]string{
			configMapKeyRef.Key: `#!/bin/bash

if [[ $(mariadb -u root -p"${MARIADB_ROOT_PASSWORD}" -e "SHOW VARIABLES LIKE 'rpl_semi_sync_slave_enabled';" --skip-column-names | grep -c "ON") -eq 1 ]]; then
	mariadb -u root -p"${MARIADB_ROOT_PASSWORD}" -e "SHOW SLAVE STATUS\G" | grep -c "Slave_IO_Running: Yes"
else
	mariadb -u root -p"${MARIADB_ROOT_PASSWORD}" -e "SELECT 1;"
fi
`,
		},
	}
	return r.configMapreconciler.Reconcile(ctx, &req)
}

func (r *ReplicationReconciler) reconcileReplication(ctx context.Context, req *reconcileRequest, logger logr.Logger) (ctrl.Result, error) {
	if req.mariadb.IsSwitchingPrimary() {
		return ctrl.Result{}, nil
	}
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	for _, i := range r.replicationPodIndexes(ctx, req) {
		if result, err := r.reconcileReplicationInPod(ctx, req, logger, i); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ReplicationReconciler) replicationPodIndexes(ctx context.Context, req *reconcileRequest) []int {
	podIndexes := []int{
		*req.mariadb.Status.CurrentPrimaryPodIndex,
	}
	for i := 0; i < int(req.mariadb.Spec.Replicas); i++ {
		if i != *req.mariadb.Status.CurrentPrimaryPodIndex {
			podIndexes = append(podIndexes, i)
		}
	}
	return podIndexes
}

func (r *ReplicationReconciler) reconcileReplicationInPod(ctx context.Context, req *reconcileRequest, logger logr.Logger,
	index int) (ctrl.Result, error) {
	primaryPodIndex := *req.mariadb.Status.CurrentPrimaryPodIndex
	replicationStatus := req.mariadb.Status.ReplicationStatus
	pod := statefulset.PodName(req.mariadb.ObjectMeta, index)

	if primaryPodIndex == index {
		if rs, ok := replicationStatus[pod]; ok && rs == mariadbv1alpha1.ReplicationStateMaster {
			return ctrl.Result{}, nil
		}

		client, err := req.clientSet.currentPrimaryClient(ctx)
		if err != nil {
			logger.V(1).Info("error getting current primary client", "err", err, "pod", pod)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		logger.Info("Configuring replication in primary", "pod", pod)
		return ctrl.Result{}, r.replConfig.ConfigurePrimary(ctx, req.mariadb, client, index)
	}

	if rs, ok := replicationStatus[pod]; ok && rs == mariadbv1alpha1.ReplicationStateSlave {
		return ctrl.Result{}, nil
	}

	client, err := req.clientSet.clientForIndex(ctx, index)
	if err != nil {
		logger.V(1).Info("error getting replica client", "err", err, "pod", pod)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	logger.Info("Configuring replication in replica", "pod", pod)
	return ctrl.Result{}, r.replConfig.ConfigureReplica(ctx, req.mariadb, client, index, primaryPodIndex, false)
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
