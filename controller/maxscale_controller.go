package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/auth"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/deployment"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/servicemonitor"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/statefulset"
	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	mxsclient "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/client"
	mxsconfig "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/config"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/predicate"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"github.com/mariadb-operator/mariadb-operator/pkg/watch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var maxScaleFinalizerName = "maxscale.k8s.mariadb.com/finalizer"

// MaxScaleReconciler reconciles a MaxScale object
type MaxScaleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	Builder        *builder.Builder
	ConditionReady *condition.Ready
	Environment    *environment.OperatorEnv
	RefResolver    *refresolver.RefResolver
	Discovery      *discovery.Discovery

	SecretReconciler         *secret.SecretReconciler
	RBACReconciler           *rbac.RBACReconciler
	AuthReconciler           *auth.AuthReconciler
	StatefulSetReconciler    *statefulset.StatefulSetReconciler
	ServiceReconciler        *service.ServiceReconciler
	DeploymentReconciler     *deployment.DeploymentReconciler
	ServiceMonitorReconciler *servicemonitor.ServiceMonitorReconciler

	SuspendEnabled bool

	RequeueInterval time.Duration
	LogMaxScale     bool
}

type requestMaxScale struct {
	mxs       *mariadbv1alpha1.MaxScale
	podClient *mxsclient.Client
}

type reconcileFnMaxScale func(context.Context, *requestMaxScale) (ctrl.Result, error)

type reconcilePhaseMaxScale struct {
	name      string
	reconcile reconcileFnMaxScale
}

//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=maxscales,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=maxscales/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=maxscales/finalizers,verbs=update
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=users;grants;connections,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list;deletecollection
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MaxScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var mxs mariadbv1alpha1.MaxScale
	if err := r.Get(ctx, req.NamespacedName, &mxs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	podClient, err := r.clientWitHealthyPod(ctx, &mxs)
	if err != nil {
		log.FromContext(ctx).V(1).Info("unable to get healthy Pod client", "err", err)
	}
	request := &requestMaxScale{
		mxs:       &mxs,
		podClient: podClient,
	}

	phases := []reconcilePhaseMaxScale{
		{
			name:      "Finalizer",
			reconcile: r.reconcileFinalizer,
		},
		{
			name:      "Spec",
			reconcile: r.setSpecDefaults,
		},
		{
			name:      "Status",
			reconcile: r.reconcileStatus,
		},
		{
			name:      "Secret",
			reconcile: r.reconcileSecret,
		},
		{
			name:      "Auth",
			reconcile: r.reconcileAuth,
		},
		{
			name:      "ServiceAccount",
			reconcile: r.reconcileServiceAccount,
		},
		{
			name:      "StatefulSet",
			reconcile: r.reconcileStatefulSet,
		},
		{
			name:      "PodDisruptionBudget",
			reconcile: r.reconcilePodDisruptionBudget,
		},
		{
			name:      "Kubernetes Service",
			reconcile: r.reconcileService,
		},
		{
			name:      "StatefulSet Ready",
			reconcile: r.ensureStatefulSetReady,
		},
		{
			name:      "Admin",
			reconcile: r.reconcileAdmin,
		},
		{
			name:      "Init",
			reconcile: r.reconcileInit,
		},
		{
			name:      "Sync",
			reconcile: r.reconcileSync,
		},
		{
			name:      "Primary Server",
			reconcile: r.ensurePrimaryServer,
		},
		{
			name:      "Servers",
			reconcile: r.reconcileServers,
		},
		{
			name:      "Monitor",
			reconcile: r.reconcileMonitor,
		},
		{
			name:      "Monitor State",
			reconcile: r.reconcileMonitorState,
		},
		{
			name:      "Services",
			reconcile: r.reconcileServices,
		},
		{
			name:      "Service State",
			reconcile: r.reconcileServiceState,
		},
		{
			name:      "Listeners",
			reconcile: r.reconcileListeners,
		},
		{
			name:      "Listener State",
			reconcile: r.reconcileListenerState,
		},
		{
			name:      "Connection",
			reconcile: r.reconcileConnection,
		},
		{
			name:      "Metrics",
			reconcile: r.reconcileMetrics,
		},
	}

	for _, p := range phases {
		result, err := p.reconcile(ctx, request)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			if err := r.handleError(ctx, &mxs, err, r.handleConfigSyncConflict); err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling phase %s: %v", p.name, err)
			}
		}
		if !result.IsZero() {
			return result, err
		}
	}

	return r.requeueResult(ctx, &mxs)
}

type errorHandler func(ctx context.Context, mxs *mariadbv1alpha1.MaxScale, err error) error

func (r *MaxScaleReconciler) handleError(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	err error, handlers ...errorHandler) error {
	var errBundle *multierror.Error
	errBundle = multierror.Append(errBundle, err)

	for _, handler := range handlers {
		handlerErr := handler(ctx, mxs, err)
		errBundle = multierror.Append(errBundle, handlerErr)
	}

	patchErr := r.patchStatus(ctx, mxs, func(s *mariadbv1alpha1.MaxScaleStatus) error {
		r.ConditionReady.PatcherFailed(err.Error())(s)
		return nil
	})
	if apierrors.IsNotFound(patchErr) {
		errBundle = multierror.Append(errBundle, patchErr)
	}

	return errBundle.ErrorOrNil()
}

func (r *MaxScaleReconciler) reconcileFinalizer(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if !req.mxs.IsBeingDeleted() {
		if !controllerutil.ContainsFinalizer(req.mxs, maxScaleFinalizerName) {

			if err := r.patch(ctx, req.mxs, func(mxs *mariadbv1alpha1.MaxScale) {
				controllerutil.AddFinalizer(req.mxs, maxScaleFinalizerName)
			}); err != nil {
				return ctrl.Result{}, fmt.Errorf("error adding finalizer: %v", err)
			}
		}
		return ctrl.Result{}, nil
	}
	var bundleErr *multierror.Error

	deleteOpts := &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			LabelSelector: klabels.SelectorFromSet(
				labels.NewLabelsBuilder().
					WithMaxScaleSelectorLabels(req.mxs).
					Build(),
			),
			Namespace: req.mxs.Namespace,
		},
	}
	if err := r.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, deleteOpts); err != nil {
		bundleErr = multierror.Append(bundleErr, fmt.Errorf("error deleting PVCs: %v", err))
	}

	if req.mxs.Spec.Config.Sync != nil {
		sql, err := r.getPrimarySqlClient(ctx, req.mxs)
		if err != nil {
			bundleErr = multierror.Append(bundleErr, fmt.Errorf("error getting primary SQL client: %v", err))
		}
		if sql != nil {
			if err := sql.DropMaxScaleConfig(ctx); err != nil {
				bundleErr = multierror.Append(bundleErr, fmt.Errorf("error dropping maxscale_config table: %v", err))
			}
		}
	}

	if err := bundleErr.ErrorOrNil(); err != nil {
		log.FromContext(ctx).Error(err, "error finalizing Maxscale")
	}

	if err := r.patch(ctx, req.mxs, func(mxs *mariadbv1alpha1.MaxScale) {
		controllerutil.RemoveFinalizer(req.mxs, maxScaleFinalizerName)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error removing finalizer: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) setSpecDefaults(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if req.mxs.Spec.MariaDBRef != nil {
		if err := r.setMariadbDefaults(ctx, req); err != nil {
			return ctrl.Result{}, fmt.Errorf("error setting MariaDB defaults: %v", err)
		}
	}
	if err := r.patch(ctx, req.mxs, func(mxs *mariadbv1alpha1.MaxScale) {
		mxs.SetDefaults(r.Environment, nil)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error setting defaults: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) setMariadbDefaults(ctx context.Context, req *requestMaxScale) error {
	mdb, err := r.getMariaDB(ctx, req)
	if err != nil {
		return err
	}
	servers := make([]mariadbv1alpha1.MaxScaleServer, mdb.Spec.Replicas)
	for i := 0; i < int(mdb.Spec.Replicas); i++ {
		name := stsobj.PodName(mdb.ObjectMeta, i)
		address := stsobj.PodFQDNWithService(mdb.ObjectMeta, i, mdb.InternalServiceKey().Name)

		var server mariadbv1alpha1.MaxScaleServer
		if i < len(req.mxs.Spec.Servers) {
			server = req.mxs.Spec.Servers[i]
		} else {
			server = mariadbv1alpha1.MaxScaleServer{
				Name:    name,
				Address: address,
				Port:    mdb.Spec.Port,
			}
		}
		servers[i] = server
	}

	monitorModule := mariadbv1alpha1.MonitorModuleMariadb
	monitorParams := map[string]string{
		"auto_failover":                "true",
		"auto_rejoin":                  "true",
		"switchover_on_low_disk_space": "true",
	}
	if mdb.IsGaleraEnabled() {
		monitorModule = mariadbv1alpha1.MonitorModuleGalera
		monitorParams = nil
	}

	return r.patch(ctx, req.mxs, func(mxs *mariadbv1alpha1.MaxScale) {
		mxs.Spec.Servers = servers
		mxs.Spec.Monitor.Module = monitorModule
		if mxs.Spec.Monitor.Params == nil {
			mxs.Spec.Monitor.Params = monitorParams
		}
		mxs.SetDefaults(r.Environment, mdb)
	})
}

func (r *MaxScaleReconciler) getMariaDB(ctx context.Context, req *requestMaxScale) (*mariadbv1alpha1.MariaDB, error) {
	if req.mxs.Spec.MariaDBRef == nil {
		return nil, errors.New("'spec.mariaDbRef' must be set")
	}
	mdb, err := r.RefResolver.MariaDB(ctx, req.mxs.Spec.MariaDBRef, req.mxs.Namespace)
	if err != nil {
		var errBundle *multierror.Error
		errBundle = multierror.Append(errBundle, err)

		patcher := r.ConditionReady.PatcherRefResolver(err, mdb)
		patchErr := r.patchStatus(ctx, req.mxs, func(mss *mariadbv1alpha1.MaxScaleStatus) error {
			patcher(mss)
			return nil
		})
		errBundle = multierror.Append(errBundle, patchErr)

		return nil, fmt.Errorf("error getting MariaDB: %v", errBundle)
	}
	return mdb, nil
}

func (r *MaxScaleReconciler) reconcileSecret(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	mxs := req.mxs
	secretKeyRef := mxs.ConfigSecretKeyRef()
	config, err := mxsconfig.Config(mxs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MaxScale config: %v", err)
	}

	secretReq := secret.SecretRequest{
		Owner:    mxs,
		Metadata: []*mariadbv1alpha1.Metadata{req.mxs.Spec.InheritMetadata},
		Key: types.NamespacedName{
			Name:      secretKeyRef.Name,
			Namespace: mxs.Namespace,
		},
		Data: map[string][]byte{
			secretKeyRef.Key: config,
		},
	}
	if err := r.SecretReconciler.Reconcile(ctx, &secretReq); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling config Secret: %v", err)
	}

	randomPasswordKeys := []mariadbv1alpha1.GeneratedSecretKeyRef{
		mxs.Spec.Auth.AdminPasswordSecretKeyRef,
		mxs.Spec.Auth.ClientPasswordSecretKeyRef,
		mxs.Spec.Auth.ServerPasswordSecretKeyRef,
		mxs.Spec.Auth.MonitorPasswordSecretKeyRef,
	}
	if mxs.Spec.Auth.SyncPasswordSecretKeyRef != nil {
		randomPasswordKeys = append(randomPasswordKeys, *mxs.Spec.Auth.SyncPasswordSecretKeyRef)
	}
	if mxs.AreMetricsEnabled() {
		randomPasswordKeys = append(randomPasswordKeys, mxs.Spec.Auth.MetricsPasswordSecretKeyRef)
	}

	for _, secretKeyRef := range randomPasswordKeys {
		if secretKeyRef.Name == "" || secretKeyRef.Key == "" {
			log.FromContext(ctx).V(1).Info("Secret not initialized. Requeuing", "secret-name", secretKeyRef.Name, "secret-key", secretKeyRef.Key)
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}

		randomSecretReq := secret.PasswordRequest{
			Owner:    mxs,
			Metadata: mxs.Spec.InheritMetadata,
			Key: types.NamespacedName{
				Name:      secretKeyRef.Name,
				Namespace: mxs.Namespace,
			},
			SecretKey: secretKeyRef.Key,
			Generate:  secretKeyRef.Generate,
		}
		if _, err := r.SecretReconciler.ReconcilePassword(ctx, randomSecretReq); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling password: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileServiceAccount(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	key := req.mxs.Spec.PodTemplate.ServiceAccountKey(req.mxs.ObjectMeta)
	_, err := r.RBACReconciler.ReconcileServiceAccount(ctx, key, req.mxs, req.mxs.Spec.InheritMetadata)
	return ctrl.Result{}, err
}

func (r *MaxScaleReconciler) reconcileStatefulSet(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	key := client.ObjectKeyFromObject(req.mxs)
	desiredSts, err := r.Builder.BuildMaxscaleStatefulSet(req.mxs, key)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building StatefulSet: %v", err)
	}
	return ctrl.Result{}, r.StatefulSetReconciler.Reconcile(ctx, desiredSts)
}

func (r *MaxScaleReconciler) reconcilePodDisruptionBudget(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	mxs := req.mxs
	if mxs.Spec.PodDisruptionBudget != nil {
		return ctrl.Result{}, r.reconcilePDBWithAvailability(
			ctx,
			mxs,
			mxs.Spec.PodDisruptionBudget.MinAvailable,
			mxs.Spec.PodDisruptionBudget.MaxUnavailable,
		)
	}
	if mxs.Spec.Replicas > 1 {
		minAvailable := intstr.FromString("50%")
		return ctrl.Result{}, r.reconcilePDBWithAvailability(
			ctx,
			mxs,
			&minAvailable,
			nil,
		)
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcilePDBWithAvailability(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale,
	minAvailable, maxUnavailable *intstr.IntOrString) error {
	key := client.ObjectKeyFromObject(maxscale)
	var existingPDB policyv1.PodDisruptionBudget
	if err := r.Get(ctx, key, &existingPDB); err == nil {
		return nil
	}

	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMaxScaleSelectorLabels(maxscale).
			Build()
	opts := builder.PodDisruptionBudgetOpts{
		Metadata:       maxscale.Spec.InheritMetadata,
		Key:            key,
		MinAvailable:   minAvailable,
		MaxUnavailable: maxUnavailable,
		SelectorLabels: selectorLabels,
	}
	pdb, err := r.Builder.BuildPodDisruptionBudget(opts, maxscale)
	if err != nil {
		return fmt.Errorf("error building PodDisruptionBudget: %v", err)
	}
	return r.Create(ctx, pdb)
}

func (r *MaxScaleReconciler) reconcileService(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if err := r.reconcileInternalService(ctx, req.mxs); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileKubernetesService(ctx, req.mxs); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, r.reconcileGuiKubernetesService(ctx, req.mxs)
}

func (r *MaxScaleReconciler) reconcileInternalService(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) error {
	key := maxscale.InternalServiceKey()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMaxScaleSelectorLabels(maxscale).
			Build()

	opts := builder.ServiceOpts{
		ExtraMeta:      maxscale.Spec.InheritMetadata,
		Headless:       true,
		SelectorLabels: selectorLabels,
	}
	desiredSvc, err := r.Builder.BuildService(key, maxscale, opts)
	if err != nil {
		return fmt.Errorf("error building internal Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MaxScaleReconciler) reconcileKubernetesService(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) error {
	key := client.ObjectKeyFromObject(maxscale)
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMaxScaleSelectorLabels(maxscale).
			Build()
	ports := []corev1.ServicePort{
		{
			Name: "admin",
			Port: int32(maxscale.Spec.Admin.Port),
		},
	}
	for _, svc := range maxscale.Spec.Services {
		ports = append(ports, corev1.ServicePort{
			Name: svc.Listener.Name,
			Port: svc.Listener.Port,
		})
	}
	opts := builder.ServiceOpts{
		ExtraMeta:      maxscale.Spec.InheritMetadata,
		Ports:          ports,
		SelectorLabels: selectorLabels,
	}
	if maxscale.Spec.KubernetesService != nil {
		opts.ServiceTemplate = *maxscale.Spec.KubernetesService
	}

	desiredSvc, err := r.Builder.BuildService(key, maxscale, opts)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MaxScaleReconciler) reconcileGuiKubernetesService(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) error {
	if !ptr.Deref(maxscale.Spec.Admin.GuiEnabled, false) {
		return nil
	}
	key := maxscale.GuiServiceKey()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMaxScaleSelectorLabels(maxscale).
			Build()
	ports := []corev1.ServicePort{
		{
			Name: "admin",
			Port: int32(maxscale.Spec.Admin.Port),
		},
	}
	opts := builder.ServiceOpts{
		ExtraMeta:      maxscale.Spec.InheritMetadata,
		Ports:          ports,
		SelectorLabels: selectorLabels,
	}
	if maxscale.Spec.GuiKubernetesService != nil {
		opts.ServiceTemplate = *maxscale.Spec.GuiKubernetesService
	}
	opts.SessionAffinity = ptr.To(corev1.ServiceAffinityClientIP)

	desiredSvc, err := r.Builder.BuildService(key, maxscale, opts)
	if err != nil {
		return fmt.Errorf("error building GUI Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MaxScaleReconciler) ensureStatefulSetReady(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(req.mxs), &sts); err != nil {
		return ctrl.Result{}, err
	}
	if r.isStatefulSetReady(&sts, req.mxs) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
}

func (r *MaxScaleReconciler) isStatefulSetReady(sts *appsv1.StatefulSet, mxs *mariadbv1alpha1.MaxScale) bool {
	return sts.Status.ReadyReplicas == sts.Status.Replicas && sts.Status.ReadyReplicas == mxs.Spec.Replicas
}

type maxscaleAuthReconcileItem struct {
	key    types.NamespacedName
	user   builder.UserOpts
	grants []auth.GrantOpts
}

func (r *MaxScaleReconciler) reconcileAuth(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	mxs := req.mxs
	// TODO: support for external databases by extending MariaDBRef
	if !ptr.Deref(mxs.Spec.Auth.Generate, false) || mxs.Spec.MariaDBRef == nil {
		return ctrl.Result{}, nil
	}

	clientKey := types.NamespacedName{
		Name:      mxs.Spec.Auth.ClientUsername,
		Namespace: mxs.Namespace,
	}
	serverKey := types.NamespacedName{
		Name:      mxs.Spec.Auth.ServerUsername,
		Namespace: mxs.Namespace,
	}
	monitorKey := types.NamespacedName{
		Name:      mxs.Spec.Auth.MonitorUsername,
		Namespace: mxs.Namespace,
	}

	items := []maxscaleAuthReconcileItem{
		{
			key: clientKey,
			user: builder.UserOpts{
				Name:                 mxs.Spec.Auth.ClientUsername,
				PasswordSecretKeyRef: &mxs.Spec.Auth.ClientPasswordSecretKeyRef.SecretKeySelector,
				MaxUserConnections:   mxs.Spec.Auth.ClientMaxConnections,
				Metadata:             mxs.Spec.InheritMetadata,
				MariaDBRef:           *mxs.Spec.MariaDBRef,
			},
			grants: []auth.GrantOpts{
				{
					Key: clientKey,
					GrantOpts: builder.GrantOpts{
						Privileges: []string{
							"SELECT",
							"INSERT",
							"UPDATE",
							"DELETE",
						},
						Database:    "*",
						Table:       "*",
						Username:    mxs.Spec.Auth.ClientUsername,
						Host:        "%",
						GrantOption: false,
						Metadata:    mxs.Spec.InheritMetadata,
						MariaDBRef:  *mxs.Spec.MariaDBRef,
					},
				},
			},
		},
		{
			key: serverKey,
			user: builder.UserOpts{
				Name:                 mxs.Spec.Auth.ServerUsername,
				PasswordSecretKeyRef: &mxs.Spec.Auth.ServerPasswordSecretKeyRef.SecretKeySelector,
				MaxUserConnections:   mxs.Spec.Auth.ServerMaxConnections,
				Metadata:             mxs.Spec.InheritMetadata,
				MariaDBRef:           *mxs.Spec.MariaDBRef,
			},
			grants: []auth.GrantOpts{
				{
					Key: types.NamespacedName{
						Name:      fmt.Sprintf("%s-mysql", serverKey.Name),
						Namespace: serverKey.Namespace,
					},
					GrantOpts: builder.GrantOpts{
						Privileges: []string{
							"SELECT",
						},
						Database:    "mysql",
						Table:       "*",
						Username:    mxs.Spec.Auth.ServerUsername,
						Host:        "%",
						GrantOption: false,
						Metadata:    mxs.Spec.InheritMetadata,
						MariaDBRef:  *mxs.Spec.MariaDBRef,
					},
				},
				{
					Key: types.NamespacedName{
						Name:      fmt.Sprintf("%s-databases", serverKey.Name),
						Namespace: serverKey.Namespace,
					},
					GrantOpts: builder.GrantOpts{
						Privileges: []string{
							"SHOW DATABASES",
						},
						Database:    "*",
						Table:       "*",
						Username:    mxs.Spec.Auth.ServerUsername,
						Host:        "%",
						GrantOption: false,
						Metadata:    mxs.Spec.InheritMetadata,
						MariaDBRef:  *mxs.Spec.MariaDBRef,
					},
				},
			},
		},
		{
			key: monitorKey,
			user: builder.UserOpts{
				Name:                 mxs.Spec.Auth.MonitorUsername,
				PasswordSecretKeyRef: &mxs.Spec.Auth.MonitorPasswordSecretKeyRef.SecretKeySelector,
				MaxUserConnections:   mxs.Spec.Auth.MonitorMaxConnections,
				Metadata:             mxs.Spec.InheritMetadata,
				MariaDBRef:           *mxs.Spec.MariaDBRef,
			},
			grants: monitorGrantOpts(monitorKey, mxs),
		},
	}
	if mxs.Spec.Config.Sync != nil && mxs.Spec.Auth.SyncUsername != nil && mxs.Spec.Auth.SyncPasswordSecretKeyRef != nil &&
		mxs.Spec.Auth.SyncMaxConnections != nil {
		syncKey := types.NamespacedName{
			Name:      *mxs.Spec.Auth.SyncUsername,
			Namespace: mxs.Namespace,
		}
		items = append(items, maxscaleAuthReconcileItem{
			key: syncKey,
			user: builder.UserOpts{
				Name:                 *mxs.Spec.Auth.SyncUsername,
				PasswordSecretKeyRef: &mxs.Spec.Auth.SyncPasswordSecretKeyRef.SecretKeySelector,
				MaxUserConnections:   *mxs.Spec.Auth.SyncMaxConnections,
				Metadata:             mxs.Spec.InheritMetadata,
				MariaDBRef:           *mxs.Spec.MariaDBRef,
			},
			grants: []auth.GrantOpts{
				{
					Key: syncKey,
					GrantOpts: builder.GrantOpts{
						Privileges: []string{
							"SELECT",
							"INSERT",
							"UPDATE",
							"CREATE",
							"DROP",
						},
						Database:    mxs.Spec.Config.Sync.Database,
						Table:       "maxscale_config",
						Username:    *mxs.Spec.Auth.SyncUsername,
						Host:        "%",
						GrantOption: false,
						Metadata:    mxs.Spec.InheritMetadata,
						MariaDBRef:  *mxs.Spec.MariaDBRef,
					},
				},
			},
		})
	}

	for _, item := range items {
		if result, err := r.AuthReconciler.ReconcileUserGrant(ctx, item.key, mxs, item.user, item.grants...); !result.IsZero() || err != nil {
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling %s user auth: %v", item.key.Name, err)
			}
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func monitorGrantOpts(key types.NamespacedName, mxs *mariadbv1alpha1.MaxScale) []auth.GrantOpts {
	if mxs.Spec.Monitor.Module == mariadbv1alpha1.MonitorModuleMariadb {
		return []auth.GrantOpts{
			{
				Key: key,
				GrantOpts: builder.GrantOpts{
					Privileges: []string{
						"BINLOG ADMIN",
						"CONNECTION ADMIN",
						"EVENT",
						"PROCESS",
						"PROCESS",
						"READ_ONLY ADMIN",
						"RELOAD",
						"REPLICA MONITOR",
						"REPLICATION CLIENT",
						"REPLICATION SLAVE ADMIN",
						"REPLICATION SLAVE",
						"SELECT",
						"SET USER",
						"SHOW DATABASES",
						"SLAVE MONITOR",
						"SUPER",
					},
					Database:    "*",
					Table:       "*",
					Username:    mxs.Spec.Auth.MonitorUsername,
					Host:        "%",
					GrantOption: false,
					Metadata:    mxs.Spec.InheritMetadata,
					MariaDBRef:  *mxs.Spec.MariaDBRef,
				},
			},
		}
	}
	return nil
}

func (r *MaxScaleReconciler) reconcileAdmin(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	result, err := r.forEachPod(ctx, req.mxs, func(podIndex int, podName string, client *mxsclient.Client) (ctrl.Result, error) {
		if err := r.reconcileAdminInPod(ctx, req.mxs, podIndex, podName, client); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling API admin in Pod '%s': %v", podName, err)
		}
		return ctrl.Result{}, nil
	})
	if !result.IsZero() || err != nil {
		return result, err
	}

	return r.reconcileMetricsAdmin(ctx, req)
}

func (r *MaxScaleReconciler) reconcileAdminInPod(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podIndex int, podName string, client *mxsclient.Client) error {
	_, err := client.User.Get(ctx, mxs.Spec.Auth.AdminUsername)
	if err == nil {
		return nil
	}
	if !mxsclient.IsUnautorized(err) && !mxsclient.IsNotFound(err) {
		return fmt.Errorf("error getting admin user: %v", err)
	}
	log.FromContext(ctx).Info("Configuring admin in MaxScale Pod", "pod", podName)

	defaultClient, err := r.defaultClientWithPodIndex(ctx, mxs, podIndex)
	if err != nil {
		return fmt.Errorf("error getting MaxScale client: %v", err)
	}
	mxsApi := newMaxScaleAPI(mxs, defaultClient, r.RefResolver)

	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.AdminPasswordSecretKeyRef.SecretKeySelector, mxs.Namespace)
	if err != nil {
		return fmt.Errorf("error getting admin password: %v", err)
	}
	if err := mxsApi.createAdminUser(ctx, mxs.Spec.Auth.AdminUsername, password); err != nil {
		return fmt.Errorf("error creating admin: %v", err)
	}
	if ptr.Deref(mxs.Spec.Auth.DeleteDefaultAdmin, false) {
		if err := defaultClient.User.DeleteDefaultAdmin(ctx); err != nil {
			return fmt.Errorf("error deleting default admin: %v", err)
		}
	}
	return nil
}

func (r *MaxScaleReconciler) reconcileMetricsAdmin(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if !req.mxs.AreMetricsEnabled() {
		return ctrl.Result{}, nil
	}

	result, err := r.forEachPod(ctx, req.mxs, func(podIndex int, podName string, client *mxsclient.Client) (ctrl.Result, error) {
		if err := r.reconcileMetricsAdminInPod(ctx, req.mxs, client); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling metrics admin in Pod '%s': %v", podName, err)
		}
		return ctrl.Result{}, nil
	})
	if !result.IsZero() || err != nil {
		return result, err
	}

	if req.podClient == nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if _, err := req.podClient.User.Get(ctx, req.mxs.Spec.Auth.MetricsUsername); err == nil {
		return ctrl.Result{}, r.patchUser(
			ctx,
			req.mxs,
			req.podClient,
			req.mxs.Spec.Auth.MetricsUsername,
			req.mxs.Spec.Auth.MetricsPasswordSecretKeyRef.SecretKeySelector,
		)
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileMetricsAdminInPod(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) error {
	_, err := client.User.Get(ctx, mxs.Spec.Auth.MetricsUsername)
	if err == nil {
		return nil
	}
	mxsApi := newMaxScaleAPI(mxs, client, r.RefResolver)

	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.MetricsPasswordSecretKeyRef.SecretKeySelector, mxs.Namespace)
	if err != nil {
		return fmt.Errorf("error getting metrics admin password: %v", err)
	}
	if err := mxsApi.createAdminUser(ctx, mxs.Spec.Auth.MetricsUsername, password); err != nil {
		return fmt.Errorf("error creating metrics admin: %v", err)
	}
	return nil
}

func (r *MaxScaleReconciler) patchUser(ctx context.Context, mxs *mariadbv1alpha1.MaxScale, client *mxsclient.Client,
	username string, passwordKeyRef corev1.SecretKeySelector) error {
	password, err := r.RefResolver.SecretKeyRef(ctx, passwordKeyRef, mxs.Namespace)
	if err != nil {
		return fmt.Errorf("error getting password: %v", err)
	}
	mxsApi := newMaxScaleAPI(mxs, client, r.RefResolver)

	return mxsApi.patchUser(ctx, username, password)
}

func (r *MaxScaleReconciler) reconcileInit(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	return r.forEachPod(ctx, req.mxs, func(podIndex int, podName string, client *mxsclient.Client) (ctrl.Result, error) {
		result, err := r.reconcileInitInPod(ctx, req.mxs, podName, client)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error initializing Pod '%s': %v", podName, err)
		}
		return result, nil
	})
}

func (r *MaxScaleReconciler) reconcileInitInPod(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podName string, client *mxsclient.Client) (ctrl.Result, error) {
	shouldInitialize, err := r.shouldInitialize(ctx, mxs, client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking initialization status: %v", err)
	}
	if !shouldInitialize {
		return ctrl.Result{}, nil
	}
	log.FromContext(ctx).Info("Initializing MaxScale Pod", "pod", podName)

	req := &requestMaxScale{
		mxs:       mxs,
		podClient: client,
	}
	reconcileFns := []reconcileFnMaxScale{
		r.reconcileServers,
		r.reconcileMonitor,
	}
	for _, reconcileFn := range reconcileFns {
		if result, err := reconcileFn(ctx, req); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) shouldInitialize(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (bool, error) {
	allExist, err := client.Server.AllExists(ctx, mxs.ServerIDs())
	if err != nil {
		return false, fmt.Errorf("error checking if all servers exist: %v", err)
	}
	if !allExist {
		return true, nil
	}
	allExist, err = client.Monitor.AllExists(ctx, []string{mxs.Spec.Monitor.Name})
	if err != nil {
		return false, fmt.Errorf("error checking if monitor exists: %v", err)
	}
	if !allExist {
		return true, nil
	}
	return false, nil
}

func (r *MaxScaleReconciler) reconcileSync(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if !req.mxs.IsHAEnabled() {
		return ctrl.Result{}, nil
	}
	return r.forEachPod(ctx, req.mxs, func(podIndex int, podName string, client *mxsclient.Client) (ctrl.Result, error) {
		isSynced, err := r.reconcileSyncInPod(ctx, req.mxs, podName, client)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling config sync in Pod '%s': %v", podName, err)
		}
		if !isSynced {
			return ctrl.Result{RequeueAfter: req.mxs.Spec.Config.Sync.Interval.Duration}, nil
		}
		return ctrl.Result{}, nil
	})
}

func (r *MaxScaleReconciler) reconcileSyncInPod(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podName string, client *mxsclient.Client) (bool, error) {
	mxsApi := newMaxScaleAPI(mxs, client, r.RefResolver)

	isSynced, err := mxsApi.isMaxScaleConfigSynced(ctx)
	if err != nil {
		return false, fmt.Errorf("error checking MaxScale config sync: %v", err)
	}
	if isSynced {
		return true, nil
	}
	log.FromContext(ctx).Info("Setting up config sync in MaxScale Pod", "pod", podName)

	return false, mxsApi.patchMaxScaleConfigSync(ctx)
}

func (r *MaxScaleReconciler) ensurePrimaryServer(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if req.mxs.Status.Servers == nil {
		return ctrl.Result{}, nil
	}
	if req.mxs.Status.GetPrimaryServer() != nil {
		return ctrl.Result{}, nil
	}
	log.FromContext(ctx).V(1).Info("No primary servers were found. Requeuing.")
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
}

func (r *MaxScaleReconciler) reconcileServers(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if req.podClient == nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	currentIdx := req.mxs.ServerIndex()
	previousIdx, err := req.podClient.Server.ListIndex(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting server index: %v", err)
	}
	diff := ds.Diff(currentIdx, previousIdx)

	if r.LogMaxScale {
		log.FromContext(ctx).V(1).Info(
			"Server diff",
			"added", diff.Added,
			"deleted", diff.Deleted,
			"rest", diff.Rest,
		)
	}
	mxsApi := newMaxScaleAPI(req.mxs, req.podClient, r.RefResolver)

	for _, id := range diff.Added {
		srv, err := ds.Get(currentIdx, id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting server to add", "server", id)
			continue
		}
		if err := mxsApi.createServer(ctx, &srv); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating server: %v", err)
		}
		if err := mxsApi.updateServerState(ctx, &srv); err != nil {
			return ctrl.Result{}, fmt.Errorf("error updating server state: %v", err)
		}
	}

	for _, id := range diff.Deleted {
		srv, err := ds.Get(previousIdx, id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting server to delete", "server", id)
			continue
		}
		if err := mxsApi.deleteServer(ctx, srv.ID); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting server: %v", err)
		}
	}

	for _, id := range diff.Rest {
		srv, err := ds.Get(currentIdx, id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting server to patch", "server", id)
			continue
		}
		if err := mxsApi.patchServer(ctx, &srv); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching server: %v", err)
		}
		if err := mxsApi.updateServerState(ctx, &srv); err != nil {
			return ctrl.Result{}, fmt.Errorf("error updating server state: %v", err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileMonitor(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if req.podClient == nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	mxsApi := newMaxScaleAPI(req.mxs, req.podClient, r.RefResolver)

	_, err := req.podClient.Monitor.Get(ctx, req.mxs.Spec.Monitor.Name)
	if err != nil {
		if !mxsclient.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting monitor: %v", err)
		}

		rels, err := mxsApi.serverRelationships(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting server relationships: %v", err)
		}
		if err := mxsApi.createMonitor(ctx, rels); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating monitor: %v", err)
		}
	} else {
		rels, err := mxsApi.serverRelationships(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting server relationships: %v", err)
		}
		if err := mxsApi.patchMonitor(ctx, rels); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching monitor: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileMonitorState(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if !r.SuspendEnabled {
		return ctrl.Result{}, nil
	}
	// MaxScale config sync does not handle object state, we need to update all Pods.
	return r.forEachPod(ctx, req.mxs, func(podIndex int, podName string, client *mxsclient.Client) (ctrl.Result, error) {
		mxsApi := newMaxScaleAPI(req.mxs, client, r.RefResolver)

		if err := mxsApi.updateMonitorState(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("error updating monitor state: %v", err)
		}
		return ctrl.Result{}, nil
	})
}

func (r *MaxScaleReconciler) reconcileServices(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if req.podClient == nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	currentIdx := req.mxs.ServiceIndex()
	previousIdx, err := req.podClient.Service.ListIndex(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting service index: %v", err)
	}
	diff := ds.Diff(currentIdx, previousIdx)

	if r.LogMaxScale {
		log.FromContext(ctx).V(1).Info(
			"Service diff",
			"added", diff.Added,
			"deleted", diff.Deleted,
			"rest", diff.Rest,
		)
	}
	mxsApi := newMaxScaleAPI(req.mxs, req.podClient, r.RefResolver)

	rels, err := mxsApi.serverRelationships(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting server relationships: %v", err)
	}

	for _, id := range diff.Added {
		svc, err := ds.Get(currentIdx, id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting service to add", "service", id)
			continue
		}
		if err := mxsApi.createService(ctx, &svc, rels); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating service: %v", err)
		}
	}

	for _, id := range diff.Deleted {
		svc, err := ds.Get(previousIdx, id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting service to delete", "service", id)
			continue
		}
		if err := mxsApi.deleteService(ctx, svc.ID); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting service: %v", err)
		}
	}

	for _, id := range diff.Rest {
		svc, err := ds.Get(currentIdx, id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting service to patch", "service", id)
			continue
		}
		if err := mxsApi.patchService(ctx, &svc, rels); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching service: %v", err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileServiceState(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if !r.SuspendEnabled {
		return ctrl.Result{}, nil
	}
	// MaxScale config sync does not handle object state, we need to update all Pods.
	return r.forEachPod(ctx, req.mxs, func(podIndex int, podName string, client *mxsclient.Client) (ctrl.Result, error) {
		mxsApi := newMaxScaleAPI(req.mxs, client, r.RefResolver)

		for _, svc := range req.mxs.Spec.Services {
			if err := mxsApi.updateServiceState(ctx, &svc); err != nil {
				return ctrl.Result{}, fmt.Errorf("error updating service state: %v", err)
			}
		}
		return ctrl.Result{}, nil
	})
}

func (r *MaxScaleReconciler) reconcileListeners(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if req.podClient == nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	currentIdx := req.mxs.ListenerIndex()
	previousIdx, err := req.podClient.Listener.ListIndex(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting listener index: %v", err)
	}
	diff := ds.Diff(currentIdx, previousIdx)

	if r.LogMaxScale {
		log.FromContext(ctx).V(1).Info(
			"Listener diff",
			"added", diff.Added,
			"deleted", diff.Deleted,
			"rest", diff.Rest,
		)
	}
	mxsApi := newMaxScaleAPI(req.mxs, req.podClient, r.RefResolver)

	for _, id := range diff.Added {
		listener, err := ds.Get(currentIdx, id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting listener to add", "listener", id)
			continue
		}
		svc, err := req.mxs.ServiceForListener(id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting service for listener", "listener", id)
			continue
		}
		if err := mxsApi.createListener(ctx, &listener, mxsApi.serviceRelationships(svc)); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating listener: %v", err)
		}
	}

	for _, id := range diff.Deleted {
		listener, err := ds.Get(previousIdx, id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting listener to delete", "listener", id)
			continue
		}
		if err := mxsApi.deleteListener(ctx, listener.ID); err != nil {
			return ctrl.Result{}, fmt.Errorf("error ")
		}
	}

	for _, id := range diff.Rest {
		listener, err := ds.Get(currentIdx, id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting listener to patch", "listener", id)
			continue
		}
		svc, err := req.mxs.ServiceForListener(id)
		if err != nil {
			log.FromContext(ctx).Error(err, "error getting service for listener", "listener", id)
			continue
		}
		if err := mxsApi.patchListener(ctx, &listener, mxsApi.serviceRelationships(svc)); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching listener: %v", err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileListenerState(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if !r.SuspendEnabled {
		return ctrl.Result{}, nil
	}
	// MaxScale config sync does not handle object state, we need to update all Pods.
	return r.forEachPod(ctx, req.mxs, func(podIndex int, podName string, client *mxsclient.Client) (ctrl.Result, error) {
		mxsApi := newMaxScaleAPI(req.mxs, client, r.RefResolver)

		for _, listener := range req.mxs.Listeners() {
			if err := mxsApi.updateListenerState(ctx, &listener); err != nil {
				return ctrl.Result{}, fmt.Errorf("error updating listener state: %v", err)
			}
		}
		return ctrl.Result{}, nil
	})
}
func (r *MaxScaleReconciler) reconcileConnection(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if req.mxs.Spec.Connection == nil {
		return ctrl.Result{}, nil
	}
	key := req.mxs.ConnectionKey()
	var existingConn mariadbv1alpha1.Connection
	if err := r.Get(ctx, key, &existingConn); err == nil {
		return ctrl.Result{}, nil
	}

	connOpts := builder.ConnectionOpts{
		Metadata:             req.mxs.Spec.InheritMetadata,
		MaxScale:             req.mxs,
		Key:                  key,
		Username:             req.mxs.Spec.Auth.ClientUsername,
		PasswordSecretKeyRef: req.mxs.Spec.Auth.ClientPasswordSecretKeyRef.SecretKeySelector,
		Template:             req.mxs.Spec.Connection,
	}
	conn, err := r.Builder.BuildConnection(connOpts, req.mxs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building Connection: %v", err)
	}
	return ctrl.Result{}, r.Create(ctx, conn)
}

func (r *MaxScaleReconciler) forEachPod(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	fn func(podIndex int, podName string, client *mxsclient.Client) (ctrl.Result, error)) (ctrl.Result, error) {

	for i := 0; i < int(mxs.Spec.Replicas); i++ {
		pod := stsobj.PodName(mxs.ObjectMeta, i)
		client, err := r.clientWithPodIndex(ctx, mxs, i)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting MaxScale client for Pod '%s': %v", pod, err)
		}

		if result, err := fn(i, pod, client); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) patch(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale,
	patcher func(*mariadbv1alpha1.MaxScale)) error {
	patch := client.MergeFrom(maxscale.DeepCopy())
	patcher(maxscale)
	return r.Patch(ctx, maxscale, patch)
}

func (r *MaxScaleReconciler) requeueResult(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	if mxs.Spec.RequeueInterval != nil {
		log.FromContext(ctx).V(1).Info("Requeuing MaxScale")
		return ctrl.Result{RequeueAfter: mxs.Spec.RequeueInterval.Duration}, nil
	}
	if r.RequeueInterval > 0 {
		log.FromContext(ctx).V(1).Info("Requeuing MaxScale")
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MaxScaleReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.MaxScale{}).
		Owns(&mariadbv1alpha1.User{}).
		Owns(&mariadbv1alpha1.Grant{}).
		Owns(&mariadbv1alpha1.Connection{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Service{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{})

	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, r.Client)
	if err := watcherIndexer.Watch(
		ctx,
		&corev1.Secret{},
		&mariadbv1alpha1.MaxScale{},
		&mariadbv1alpha1.MaxScaleList{},
		mariadbv1alpha1.MaxScaleMetricsPasswordSecretFieldPath,
		ctrlbuilder.WithPredicates(
			predicate.PredicateWithLabel(metadata.WatchLabel),
		),
	); err != nil {
		return fmt.Errorf("error watching: %v", err)
	}

	return builder.Complete(r)
}
