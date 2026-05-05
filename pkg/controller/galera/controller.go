package galera

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	agentclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/client"
	agenterrors "github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/errors"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/pvc"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
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
	kubeClientset       *kubernetes.Clientset
	recorder            events.EventRecorder
	env                 *environment.OperatorEnv
	builder             *builder.Builder
	topologyManager     *replication.TopologyManager
	refResolver         *refresolver.RefResolver
	configMapReconciler *configmap.ConfigMapReconciler
	serviceReconciler   *service.ServiceReconciler
	pvcReconciler       *pvc.PVCReconciler
}

func NewGaleraReconciler(client client.Client, kubeClientset *kubernetes.Clientset, recorder events.EventRecorder,
	env *environment.OperatorEnv, builder *builder.Builder, topologyManager *replication.TopologyManager,
	opts ...Option) *GaleraReconciler {
	r := &GaleraReconciler{
		Client:          client,
		kubeClientset:   kubeClientset,
		recorder:        recorder,
		env:             env,
		builder:         builder,
		topologyManager: topologyManager,
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
	if r.pvcReconciler == nil {
		r.pvcReconciler = pvc.NewPVCReconciler(client)
	}
	return r
}

func shouldReconcileSwitchover(mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb.IsMaxScaleEnabled() || mdb.IsRestoringBackup() || mdb.IsUpdating() || mdb.IsResizingStorage() {
		return false
	}
	if mdb.Status.CurrentPrimaryPodIndex == nil {
		return false
	}
	currentPodIndex := ptr.Deref(mdb.Status.CurrentPrimaryPodIndex, 0)
	desiredPodIndex := ptr.Deref(ptr.Deref(mdb.Spec.Galera, mariadbv1alpha1.Galera{}).Primary.PodIndex, 0)
	return currentPodIndex != desiredPodIndex
}

func (r *GaleraReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsGaleraEnabled() {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).WithName("galera")

	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(1).Info("'status.currentPrimaryPodIndex' must be set. Skipping")
		return ctrl.Result{}, nil
	}

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		return ctrl.Result{}, err
	}
	topology := r.topologyManager.TopologyForMariaDB(mariadb, logger)

	if mariadb.HasGaleraNotReadyCondition() {
		if result, err := r.reconcileRecovery(ctx, mariadb, logger.WithName("recovery")); !result.IsZero() || err != nil {
			return result, err
		}
	}

	if !mariadb.HasGaleraReadyCondition() && sts.Status.ReadyReplicas == mariadb.Spec.Replicas {
		if err := r.disableBootstrap(ctx, mariadb, logger); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("Galera cluster is healthy")
		r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonGaleraClusterHealthy,
			mariadbv1alpha1.ActionReconciling, "Galera cluster is healthy")

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			condition.SetGaleraReady(&mariadb.Status)
			condition.SetGaleraConfigured(&mariadb.Status)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Galera status: %v", err)
		}
	}

	if mariadb.IsMultiClusterEnabled() {
		if result, err := r.reconcileMultiCluster(ctx, mariadb, topology, logger); !result.IsZero() || err != nil {
			return result, err
		}
	}

	if shouldReconcileSwitchover(mariadb) {
		primary := *mariadb.Status.CurrentPrimaryPodIndex
		newPrimary := ptr.Deref(ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).Primary.PodIndex, 0)

		logger.Info("Switching primary replica", "primary", primary, "new-primary", newPrimary)
		r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitching, mariadbv1alpha1.ActionReconciling,
			"Switching primary replica from index '%d' to index '%d'", primary, newPrimary)

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			status.UpdateCurrentPrimary(mariadb, newPrimary)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching current primary status: %v", err)
		}
		logger.Info("Primary switched", "primary", primary, "new-primary", newPrimary)
		r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitched, mariadbv1alpha1.ActionReconciling,
			"Primary switched from index '%d' to index '%d'", primary, newPrimary)

		if mariadb.IsMultiClusterEnabled() {
			// Requeue to trigger a multi-cluster reconciliation based on the new primary
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *GaleraReconciler) disableBootstrap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	logger.V(1).Info("Disabling Galera bootstrap")

	clientSet, err := agentclient.NewClientSet(ctx, mariadb, r.env, r.refResolver)
	if err != nil {
		return fmt.Errorf("error creating agent client set: %v", err)
	}
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		agentClient, err := clientSet.ClientForIndex(i)
		if err != nil {
			return fmt.Errorf("error creating agent client: %v", err)
		}
		if err := agentClient.Galera.DisableBootstrap(ctx); err != nil && !agenterrors.IsNotFound(err) {
			return fmt.Errorf("error disabling bootstrap in Pod %d: %v", i, err)
		}
	}
	return nil
}

func (r *GaleraReconciler) reconcileMultiCluster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	topology replication.Topology, logger logr.Logger) (ctrl.Result, error) {
	if !shouldReconcileMultiCluster(mariadb, logger) {
		return ctrl.Result{}, nil
	}
	clientSet := sql.NewClientSet(mariadb, r.refResolver)
	defer clientSet.Close()

	podIndexes, err := multiClusterPodIndexes(mariadb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting multi-cluster Pod indexes: %v", err)
	}
	currentPrimaryPodIndex := *mariadb.Status.CurrentPrimaryPodIndex
	logger.Info("Configuring Galera primary replica", "pod-index", currentPrimaryPodIndex)

	for _, i := range podIndexes {
		if currentPrimaryPodIndex == i {
			primaryClient, err := clientSet.ClientForIndex(ctx, i)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error getting client for current primary Pod index %d: %v", i, err)
			}
			if err := topology.ConfigurePrimary(
				ctx,
				primaryClient,
			); err != nil {
				return ctrl.Result{}, fmt.Errorf("error configuring primary replica: %v", err)
			}
		} else {
			replicaClient, err := clientSet.ClientForIndex(ctx, i)
			if err != nil {
				// During failover, operator might not be able to connect, therefore blocking the reconciliation here.
				// Leave the reset for the next reconciliation.
				logger.V(1).Info("error getting client for current Pod index", "err", "pod-index", i)
				continue
			}
			if err := replicaClient.StopSlave(
				ctx,
				sql.WithConnectionName(replication.MultiClusterReplicaConnectionName),
			); err != nil && !sql.IsConnectionNotExists(err) {
				return ctrl.Result{}, fmt.Errorf("error stopping replica slave: %v", err)
			}
			if err := replicaClient.ResetSlave(
				ctx,
				sql.WithConnectionName(replication.MultiClusterReplicaConnectionName),
			); err != nil && !sql.IsConnectionNotExists(err) {
				return ctrl.Result{}, fmt.Errorf("error resetting replica slave: %v", err)
			}
		}
	}
	logger.Info("Galera primary replica configured", "pod-index", currentPrimaryPodIndex)
	r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonMultiClusterConfigured, mariadbv1alpha1.ActionReconciling,
		"Galera primary replica configured on Pod %d", currentPrimaryPodIndex)
	// Requeue to update status
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
}

func shouldReconcileMultiCluster(mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) bool {
	if !mariadb.IsMultiClusterReplica() || !mariadb.HasGaleraConfiguredCondition() || mariadb.HasGaleraNotReadyCondition() {
		return false
	}
	if mariadb.Status.CurrentPrimary != nil && mariadb.Status.Replication != nil {
		role, ok := mariadb.Status.Replication.Roles[*mariadb.Status.CurrentPrimary]
		if !ok {
			logger.V(1).Info("Primary replica role not found. Reconciling Galera multi-cluster...")
			return true
		}
		if role != mariadbv1alpha1.ReplicationRolePrimaryReplica {
			logger.V(1).Info("Unexpected role in primary replica. Reconciling Galera multi-cluster", "role", role)
			return true
		}
	}
	return false
}

func multiClusterPodIndexes(mariadb *mariadbv1alpha1.MariaDB) ([]int, error) {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	podIndexes := []int{
		*mariadb.Status.CurrentPrimaryPodIndex,
	}
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i != *mariadb.Status.CurrentPrimaryPodIndex {
			podIndexes = append(podIndexes, i)
		}
	}
	return podIndexes, nil
}

func (r *GaleraReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
