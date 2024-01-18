package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/statefulset"
	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/maxscale"
	mxsclient "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MaxScaleReconciler reconciles a MaxScale object
type MaxScaleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	Builder        *builder.Builder
	ConditionReady *condition.Ready
	Environment    *environment.Environment
	RefResolver    *refresolver.RefResolver

	SecretReconciler      *secret.SecretReconciler
	StatefulSetReconciler *statefulset.StatefulSetReconciler
	ServiceReconciler     *service.ServiceReconciler

	RequeueInterval time.Duration
	LogRequests     bool
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=maxscales,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=maxscales/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=maxscales/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MaxScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var maxscale mariadbv1alpha1.MaxScale
	if err := r.Get(ctx, req.NamespacedName, &maxscale); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if err := r.patchStatus(ctx, &maxscale, r.patcher(ctx, &maxscale)); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	phases := []struct {
		name      string
		reconcile func(context.Context, *mariadbv1alpha1.MaxScale) (ctrl.Result, error)
	}{
		{
			name:      "Spec",
			reconcile: r.setSpecDefaults,
		},
		{
			name:      "Secret",
			reconcile: r.reconcileSecret,
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
			name:      "Admin",
			reconcile: r.reconcileAdmin,
		},
		{
			name:      "Init",
			reconcile: r.reconcileInit,
		},
		{
			name:      "Server",
			reconcile: r.reconcileServers,
		},
		{
			name:      "Monitor",
			reconcile: r.reconcileMonitor,
		},
		{
			name:      "Services",
			reconcile: r.reconcileServices,
		},
		{
			name:      "Listeners",
			reconcile: r.reconcileListeners,
		},
	}

	for _, p := range phases {
		result, err := p.reconcile(ctx, &maxscale)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			var errBundle *multierror.Error
			errBundle = multierror.Append(errBundle, err)

			msg := fmt.Sprintf("Error reconciling %s: %v", p.name, err)
			patchErr := r.patchStatus(ctx, &maxscale, func(s *mariadbv1alpha1.MaxScaleStatus) error {
				patcher := r.ConditionReady.PatcherFailed(msg)
				patcher(s)
				return nil
			})
			if apierrors.IsNotFound(patchErr) {
				errBundle = multierror.Append(errBundle, patchErr)
			}

			if err := errBundle.ErrorOrNil(); err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling %s: %v", p.name, err)
			}
		}
		if !result.IsZero() {
			return result, err
		}
	}

	return r.requeueResult(ctx, &maxscale)
}

func (r *MaxScaleReconciler) setSpecDefaults(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	return ctrl.Result{}, r.patch(ctx, maxscale, func(mxs *mariadbv1alpha1.MaxScale) {
		mxs.SetDefaults(r.Environment)
	})
}
func (r *MaxScaleReconciler) reconcileSecret(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	secretKeyRef := mxs.ConfigSecretKeyRef()
	config, err := maxscale.Config(mxs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MaxScale config: %v", err)
	}

	secretReq := secret.SecretRequest{
		Owner: mxs,
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

	randomPasswordKeys := []corev1.SecretKeySelector{
		mxs.Spec.Auth.AdminPasswordSecretKeyRef,
		mxs.Spec.Auth.ClientPasswordSecretKeyRef,
		mxs.Spec.Auth.ServerPasswordSecretKeyRef,
		mxs.Spec.Auth.MonitorPasswordSecretKeyRef,
	}
	for _, secretKeyRef := range randomPasswordKeys {
		randomSecretReq := &secret.RandomPasswordRequest{
			Owner: mxs,
			Key: types.NamespacedName{
				Name:      secretKeyRef.Name,
				Namespace: mxs.Namespace,
			},
			SecretKey: secretKeyRef.Key,
		}
		if _, err := r.SecretReconciler.ReconcileRandomPassword(ctx, randomSecretReq); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling password: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileStatefulSet(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	key := client.ObjectKeyFromObject(maxscale)
	desiredSts, err := r.Builder.BuildMaxscaleStatefulSet(maxscale, key)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building StatefulSet: %v", err)
	}
	return ctrl.Result{}, r.StatefulSetReconciler.Reconcile(ctx, desiredSts)
}

func (r *MaxScaleReconciler) reconcilePodDisruptionBudget(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	if maxscale.Spec.PodDisruptionBudget != nil {
		return ctrl.Result{}, r.reconcilePDBWithAvailability(
			ctx,
			maxscale,
			maxscale.Spec.PodDisruptionBudget.MinAvailable,
			maxscale.Spec.PodDisruptionBudget.MaxUnavailable,
		)
	}
	if maxscale.Spec.Replicas > 1 {
		minAvailable := intstr.FromString("50%")
		return ctrl.Result{}, r.reconcilePDBWithAvailability(
			ctx,
			maxscale,
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
		Key:            key,
		MinAvailable:   minAvailable,
		MaxUnavailable: maxUnavailable,
		SelectorLabels: selectorLabels,
	}
	pdb, err := r.Builder.BuildPodDisruptionBudget(&opts, maxscale)
	if err != nil {
		return fmt.Errorf("error building PodDisruptionBudget: %v", err)
	}
	return r.Create(ctx, pdb)
}

func (r *MaxScaleReconciler) reconcileService(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	if err := r.reconcileInternalService(ctx, maxscale); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, r.reconcileKubernetesService(ctx, maxscale)
}

func (r *MaxScaleReconciler) reconcileInternalService(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) error {
	key := maxscale.InternalServiceKey()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMaxScaleSelectorLabels(maxscale).
			Build()

	opts := builder.ServiceOpts{
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
		Ports:          ports,
		SelectorLabels: selectorLabels,
	}
	if maxscale.Spec.KubernetesService != nil {
		opts.ServiceTemplate = *maxscale.Spec.KubernetesService
	}

	desiredSvc, err := r.Builder.BuildService(key, maxscale, opts)
	if err != nil {
		return fmt.Errorf("error building exporter Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MaxScaleReconciler) reconcileAdmin(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	if !mxs.IsReady() {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// TODO: all Pods in order to support HA
	client, err := r.clientWithPodIndex(ctx, mxs, 0)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MaxScale client: %v", err)
	}
	_, err = client.User.Get(ctx, mxs.Spec.Auth.AdminUsername)
	if err == nil {
		return ctrl.Result{}, nil
	}
	if !mxsclient.IsUnautorized(err) && !mxsclient.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("error getting admin user: %v", err)
	}

	// TODO: all Pods in order to support HA
	defaultClient, err := r.defaultClientWithPodIndex(ctx, mxs, 0)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MaxScale client: %v", err)
	}
	mxsApi := newMaxScaleAPI(mxs, defaultClient, r.RefResolver)

	if result, err := mxsApi.createAdminUser(ctx); !result.IsZero() || err != nil {
		return result, err
	}
	if mxs.Spec.Auth.ShouldDeleteDefaultAdmin() {
		if err := defaultClient.User.DeleteDefaultAdmin(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting default admin: %v", err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileInit(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	if !mxs.IsReady() {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	// TODO: all Pods in order to support HA
	pod := stsobj.PodName(mxs.ObjectMeta, 0)
	client, err := r.clientWithPodIndex(ctx, mxs, 0)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MaxScale client for Pod '%s': %v", pod, err)
	}

	shouldInitialize, err := r.shouldInitialize(ctx, mxs, client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking initialization status in Pod '%s': %v", pod, err)
	}
	if !shouldInitialize {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx).WithValues("pod", pod)
	logger.Info("Initializing MaxScale instance")

	if result, err := r.reconcileServersWithClient(ctx, mxs, client); !result.IsZero() || err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileMonitorWithClient(ctx, mxs, client); !result.IsZero() || err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileServicesWithClient(ctx, mxs, client); !result.IsZero() || err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileListenersWithClient(ctx, mxs, client); !result.IsZero() || err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) shouldInitialize(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (bool, error) {
	anyExist, err := client.Server.AnyExists(ctx, mxs.ServerIDs())
	if err != nil {
		return false, fmt.Errorf("error checking if servers already exist: %v", err)
	}
	if anyExist {
		return false, nil
	}
	anyExist, err = client.Service.AnyExists(ctx, mxs.ServiceIDs())
	if err != nil {
		return false, fmt.Errorf("error checking if services already exist: %v", err)
	}
	if anyExist {
		return false, nil
	}
	anyExist, err = client.Service.AnyExists(ctx, mxs.ListenerIDs())
	if err != nil {
		return false, fmt.Errorf("error checking if listeners already exist: %v", err)
	}
	if anyExist {
		return false, nil
	}
	anyExist, err = client.Monitor.AnyExists(ctx, []string{mxs.Spec.Monitor.Name})
	if err != nil {
		return false, fmt.Errorf("error checking if monitors already exist: %v", err)
	}
	if anyExist {
		return false, nil
	}
	return true, nil
}

func (r *MaxScaleReconciler) reconcileServers(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	// TODO: shared client pointing to the same instance?
	client, err := r.client(ctx, mxs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting client: %v", err)
	}
	return r.reconcileServersWithClient(ctx, mxs, client)
}

func (r *MaxScaleReconciler) reconcileServersWithClient(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (ctrl.Result, error) {
	currentIdx := mxs.ServerIndex()
	previousIdx, err := client.Server.ListIndex(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting server index: %v", err)
	}
	diff := ds.Diff[
		mariadbv1alpha1.MaxScaleServer,
		mxsclient.Data[mxsclient.ServerAttributes],
	](currentIdx, previousIdx)

	logger := log.FromContext(ctx)
	logger.V(1).Info(
		"Server diff",
		"added", diff.Added,
		"deleted", diff.Deleted,
		"rest", diff.Rest,
	)
	mxsApi := newMaxScaleAPI(mxs, client, r.RefResolver)

	for _, id := range diff.Added {
		srv, err := ds.Get(currentIdx, id)
		if err != nil {
			logger.Error(err, "error getting server to add", "server", id)
			continue
		}
		if result, err := mxsApi.createServer(ctx, &srv); !result.IsZero() || err != nil {
			return result, err
		}
	}

	for _, id := range diff.Deleted {
		srv, err := ds.Get(previousIdx, id)
		if err != nil {
			logger.Error(err, "error getting server to delete", "server", id)
			continue
		}
		if result, err := mxsApi.deleteServer(ctx, srv.ID); !result.IsZero() || err != nil {
			return result, err
		}
	}

	for _, id := range diff.Rest {
		srv, err := ds.Get(currentIdx, id)
		if err != nil {
			logger.Error(err, "error getting server to patch", "server", id)
			continue
		}
		if result, err := mxsApi.patchServer(ctx, &srv); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileMonitor(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	// TODO: shared client pointing to the same instance?
	client, err := r.client(ctx, mxs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting client: %v", err)
	}
	return r.reconcileMonitorWithClient(ctx, mxs, client)
}

func (r *MaxScaleReconciler) reconcileMonitorWithClient(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (ctrl.Result, error) {
	mxsApi := newMaxScaleAPI(mxs, client, r.RefResolver)

	_, err := client.Monitor.Get(ctx, mxs.Spec.Monitor.Name)
	if err != nil {
		if !mxsclient.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting monitor: %v", err)
		}

		rels, err := mxsApi.serverRelationships(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting server relationships: %v", err)
		}
		return mxsApi.createMonitor(ctx, rels)
	}

	rels, err := mxsApi.serverRelationships(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting server relationships: %v", err)
	}
	return mxsApi.patchMonitor(ctx, rels)
}

func (r *MaxScaleReconciler) reconcileServices(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	// TODO: shared client pointing to the same instance?
	client, err := r.client(ctx, mxs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting client: %v", err)
	}
	return r.reconcileServicesWithClient(ctx, mxs, client)
}

func (r *MaxScaleReconciler) reconcileServicesWithClient(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (ctrl.Result, error) {
	currentIdx := mxs.ServiceIndex()
	previousIdx, err := client.Service.ListIndex(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting service index: %v", err)
	}
	diff := ds.Diff[
		mariadbv1alpha1.MaxScaleService,
		mxsclient.Data[mxsclient.ServiceAttributes],
	](currentIdx, previousIdx)

	logger := log.FromContext(ctx)
	logger.V(1).Info(
		"Service diff",
		"added", diff.Added,
		"deleted", diff.Deleted,
		"rest", diff.Rest,
	)
	mxsApi := newMaxScaleAPI(mxs, client, r.RefResolver)

	rels, err := mxsApi.serverRelationships(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting server relationships: %v", err)
	}

	for _, id := range diff.Added {
		svc, err := ds.Get(currentIdx, id)
		if err != nil {
			logger.Error(err, "error getting service to add", "service", id)
			continue
		}
		if result, err := mxsApi.createService(ctx, &svc, rels); !result.IsZero() || err != nil {
			return result, err
		}
	}

	for _, id := range diff.Deleted {
		svc, err := ds.Get(previousIdx, id)
		if err != nil {
			logger.Error(err, "error getting service to delete", "service", id)
			continue
		}
		if result, err := mxsApi.deleteService(ctx, svc.ID); !result.IsZero() || err != nil {
			return result, err
		}
	}

	for _, id := range diff.Rest {
		svc, err := ds.Get(currentIdx, id)
		if err != nil {
			logger.Error(err, "error getting service to patch", "service", id)
			continue
		}
		if result, err := mxsApi.patchService(ctx, &svc, rels); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileListeners(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	// TODO: shared client pointing to the same instance?
	client, err := r.client(ctx, mxs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting client: %v", err)
	}
	return r.reconcileListenersWithClient(ctx, mxs, client)
}

func (r *MaxScaleReconciler) reconcileListenersWithClient(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (ctrl.Result, error) {
	currentIdx := mxs.ListenerIndex()
	previousIdx, err := client.Listener.ListIndex(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting listener index: %v", err)
	}
	diff := ds.Diff[
		mariadbv1alpha1.MaxScaleListener,
		mxsclient.Data[mxsclient.ListenerAttributes],
	](currentIdx, previousIdx)

	logger := log.FromContext(ctx)
	logger.V(1).Info(
		"Listener diff",
		"added", diff.Added,
		"deleted", diff.Deleted,
		"rest", diff.Rest,
	)
	mxsApi := newMaxScaleAPI(mxs, client, r.RefResolver)

	for _, id := range diff.Added {
		listener, err := ds.Get(currentIdx, id)
		if err != nil {
			logger.Error(err, "error getting listener to add", "listener", id)
			continue
		}
		svc, err := mxs.ServiceForListener(id)
		if err != nil {
			logger.Error(err, "error getting service for listener", "listener", id)
			continue
		}
		if result, err := mxsApi.createListener(ctx, &listener, mxsApi.serviceRelationships(svc)); !result.IsZero() || err != nil {
			return result, err
		}
	}

	for _, id := range diff.Deleted {
		listener, err := ds.Get(previousIdx, id)
		if err != nil {
			logger.Error(err, "error getting listener to delete", "listener", id)
			continue
		}
		if result, err := mxsApi.deleteListener(ctx, listener.ID); !result.IsZero() || err != nil {
			return result, err
		}
	}

	for _, id := range diff.Rest {
		listener, err := ds.Get(currentIdx, id)
		if err != nil {
			logger.Error(err, "error getting listener to patch", "listener", id)
			continue
		}
		svc, err := mxs.ServiceForListener(id)
		if err != nil {
			logger.Error(err, "error getting service for listener", "listener", id)
			continue
		}
		if result, err := mxsApi.patchListener(ctx, &listener, mxsApi.serviceRelationships(svc)); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) patcher(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) func(*mariadbv1alpha1.MaxScaleStatus) error {
	return func(mss *mariadbv1alpha1.MaxScaleStatus) error {
		var sts appsv1.StatefulSet
		if err := r.Get(ctx, client.ObjectKeyFromObject(maxscale), &sts); err != nil {
			return err
		}
		maxscale.Status.Replicas = sts.Status.ReadyReplicas

		client, err := r.client(ctx, maxscale)
		if err != nil {
			return fmt.Errorf("error getting MaxScale client: %v", err)
		}
		masterServer, err := client.Server.GetMaster(ctx)
		if err != nil && !errors.Is(err, mxsclient.ErrMasterServerNotFound) {
			log.FromContext(ctx).V(1).Info("error getting primary server", "err", err)
		}
		if err == nil && masterServer != "" {
			if maxscale.Status.PrimaryServer != nil && *maxscale.Status.PrimaryServer != masterServer {
				fromServer := *maxscale.Status.PrimaryServer
				toServer := masterServer
				log.FromContext(ctx).Info(
					"MaxScale primary server changed",
					"from-server", fromServer,
					"to-server", toServer,
				)
				r.Recorder.Event(
					maxscale,
					corev1.EventTypeNormal,
					mariadbv1alpha1.ReasonMaxScalePrimaryServerChanged,
					fmt.Sprintf("MaxScale primary server changed from '%s' to '%s'", fromServer, toServer),
				)
			}
			maxscale.Status.PrimaryServer = &masterServer
		}

		condition.SetReadyWithStatefulSet(&maxscale.Status, &sts)
		return nil
	}
}

func (r *MaxScaleReconciler) patchStatus(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale,
	patcher func(*mariadbv1alpha1.MaxScaleStatus) error) error {
	patch := client.MergeFrom(maxscale.DeepCopy())
	if err := patcher(&maxscale.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, maxscale, patch)
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
func (r *MaxScaleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.MaxScale{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Service{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
