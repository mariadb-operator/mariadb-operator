package galera

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	agentclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/client"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/errors"
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

		if mariadb.IsMultiClusterEnabled() {
			if err := r.reconcileMultiCluster(ctx, mariadb, topology, logger); err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling multi-cluster: %v", err)
			}
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
		if mariadb.IsMultiClusterEnabled() {
			if err := r.reconcileMultiClusterSwitchover(ctx, mariadb, topology, primary, newPrimary); err != nil {
				return ctrl.Result{}, fmt.Errorf("error performingr replication switchover: %v", err)
			}
		}

		logger.Info("Primary switched", "primary", primary, "new-primary", newPrimary)
		r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitched, mariadbv1alpha1.ActionReconciling,
			"Primary switched from index '%d' to index '%d'", primary, newPrimary)
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
		if err := agentClient.Galera.DisableBootstrap(ctx); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("error disabling bootstrap in Pod %d: %v", i, err)
		}
	}
	return nil
}

func (r *GaleraReconciler) reconcileMultiCluster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	topology replication.Topology, logger logr.Logger) error {
	if !mariadb.IsMultiClusterReplica() || mariadb.HasConfiguredMultiCluster() {
		return nil
	}
	logger.Info("Configuring Galera multi-cluster replication")
	currentPrimaryPodIndex := *mariadb.Status.CurrentPrimaryPodIndex

	sqlClient, err := sql.NewInternalClientWithPodIndex(
		ctx,
		mariadb,
		r.refResolver,
		currentPrimaryPodIndex,
	)
	if err != nil {
		return fmt.Errorf("error getting SQL client for primary: %v", err)
	}
	defer sqlClient.Close()

	err = topology.ConfigurePrimary(
		ctx,
		sqlClient,
	)
	if err != nil {
		return fmt.Errorf("error configuring Galera multi-cluster replication: %v", err)
	}

	logger.Info("Galera multi-cluster replication has been configured")
	r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonMultiClusterConfigured, mariadbv1alpha1.ActionReconciling,
		"Galera multi-cluster replication has been configured")

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		condition.SetMultiClusterConfigured(&mariadb.Status)
	}); err != nil {
		return fmt.Errorf("error patching multi-cluster status: %v", err)
	}
	return nil
}

func (r *GaleraReconciler) reconcileMultiClusterSwitchover(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	topology replication.Topology, primary, newPrimary int) error {
	if !mariadb.IsMultiClusterReplica() || !mariadb.HasConfiguredMultiCluster() {
		return nil
	}
	clientSet := sql.NewClientSet(mariadb, r.refResolver)
	defer clientSet.Close()

	primaryClient, err := clientSet.ClientForIndex(ctx, primary)
	if err != nil {
		return fmt.Errorf("error getting client for current primary Pod index %d: %v", primary, err)
	}
	if err := primaryClient.StopSlave(
		ctx,
		sql.WithConnectionName(replication.MultiClusterReplicaConnectionName),
	); !sql.IsConnectionNotExists(err) {
		return fmt.Errorf("error stopping primary replica slave: %v", err)
	}
	if err := primaryClient.ResetSlave(
		ctx,
		sql.WithConnectionName(replication.MultiClusterReplicaConnectionName),
	); !sql.IsConnectionNotExists(err) {
		return fmt.Errorf("error resetting primary replica slave: %v", err)
	}

	newPrimaryClient, err := clientSet.ClientForIndex(ctx, newPrimary)
	if err != nil {
		return fmt.Errorf("error getting client for new primary Pod index %d: %v", primary, err)
	}
	if err := topology.ConfigurePrimary(ctx, newPrimaryClient); err != nil {
		return fmt.Errorf("error configuring new primary at Pod index %d: %v", newPrimary, err)
	}
	return nil
}

func (r *GaleraReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
