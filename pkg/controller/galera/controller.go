package galera

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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
	recorder            record.EventRecorder
	env                 *environment.OperatorEnv
	builder             *builder.Builder
	refResolver         *refresolver.RefResolver
	configMapReconciler *configmap.ConfigMapReconciler
	serviceReconciler   *service.ServiceReconciler
}

func NewGaleraReconciler(client client.Client, recorder record.EventRecorder, env *environment.OperatorEnv, builder *builder.Builder,
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

func (r *GaleraReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsGaleraEnabled() || mariadb.IsRestoringBackup() {
		return ctrl.Result{}, nil
	}
	sts, err := r.statefulSet(ctx, mariadb)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger := log.FromContext(ctx).WithName("galera")

	if mariadb.HasGaleraNotReadyCondition() {
		if err := r.reconcileRecovery(ctx, mariadb, sts, logger.WithName("recovery")); err != nil {
			return ctrl.Result{}, err
		}
	}

	if !mariadb.HasGaleraReadyCondition() && sts.Status.ReadyReplicas == mariadb.Spec.Replicas {
		if err := r.disableBootstrap(ctx, mariadb, logger); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("Galera cluster is healthy")
		r.recorder.Event(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonGaleraClusterHealthy, "Galera cluster is healthy")

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			condition.SetGaleraReady(&mariadb.Status)
			condition.SetGaleraConfigured(&mariadb.Status)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Galera status: %v", err)
		}
	}

	if !mariadb.HasGaleraConfiguredCondition() {
		if result, err := r.initBootstrapPod(ctx, mariadb, logger); !result.IsZero() || err != nil {
			return result, err
		}
	}

	if shouldReconcileSwitchover(mariadb) {
		fromIndex := *mariadb.Status.CurrentPrimaryPodIndex
		toIndex := ptr.Deref(ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).Primary.PodIndex, 0)

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			status.UpdateCurrentPrimary(mariadb, toIndex)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching current primary status: %v", err)
		}

		logger.Info("Primary switched", "from-index", fromIndex, "to-index", toIndex)
		r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitched,
			"Primary switched from index '%d' to index '%d'", fromIndex, toIndex)
	}
	return ctrl.Result{}, nil
}

func (r *GaleraReconciler) statefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*appsv1.StatefulSet, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		return nil, err
	}
	return &sts, nil
}

func (r GaleraReconciler) initBootstrapPod(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) (ctrl.Result, error) {
	bootstrapPodIndex := 0
	bootstrapPodKey := types.NamespacedName{
		Name:      stsobj.PodName(mariadb.ObjectMeta, bootstrapPodIndex),
		Namespace: mariadb.Namespace,
	}
	var bootstrapPod corev1.Pod
	if err := r.Get(ctx, bootstrapPodKey, &bootstrapPod); err != nil {
		logger.V(1).Info("Error getting bootstrap Pod", "pod", bootstrapPod.Name, "err", err)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	clientSet, err := r.newAgentClientSet(mariadb, mdbhttp.WithTimeout(1*time.Second))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating agent client set: %v", err)
	}
	client, err := clientSet.clientForIndex(bootstrapPodIndex)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting agent client: %v", err)
	}

	bootstrapEnabled, err := client.Bootstrap.IsEnabled(ctx)
	if err != nil {
		logger.V(1).Info("Error checking bootstrap", "err", err)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if bootstrapEnabled {
		return ctrl.Result{}, nil
	}

	isInit, err := client.State.IsMariaDBInit(ctx)
	if err != nil {
		logger.V(1).Info("Error checking MariaDB init state", "err", err)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if !isInit {
		logger.V(1).Info("MariaDB not initialized. Requeuing")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	logger.V(1).Info("Restarting bootstrap Pod")
	if err := r.Delete(ctx, &bootstrapPod); err != nil {
		return ctrl.Result{}, fmt.Errorf("error restarting bootstrap Pod: %v", err)
	}
	return ctrl.Result{}, nil
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
		if err := agentClient.Bootstrap.Disable(ctx); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("error disabling bootstrap in Pod %d: %v", i, err)
		}
	}
	return nil
}

func (r *GaleraReconciler) newAgentClientSet(mariadb *mariadbv1alpha1.MariaDB, clientOpts ...mdbhttp.Option) (*agentClientSet, error) {
	opts := []mdbhttp.Option{}
	opts = append(opts, clientOpts...)

	agent := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).Agent
	if ptr.Deref(agent.KubernetesAuth, mariadbv1alpha1.KubernetesAuth{}).Enabled {
		opts = append(opts,
			mdbhttp.WithKubernetesAuth(r.env.MariadbOperatorSAPath),
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

func shouldReconcileSwitchover(mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb.IsMaxScaleEnabled() || mdb.Status.CurrentPrimaryPodIndex == nil {
		return false
	}
	currentPodIndex := ptr.Deref(mdb.Status.CurrentPrimaryPodIndex, 0)
	desiredPodIndex := ptr.Deref(ptr.Deref(mdb.Spec.Galera, mariadbv1alpha1.Galera{}).Primary.PodIndex, 0)
	return currentPodIndex != desiredPodIndex
}
