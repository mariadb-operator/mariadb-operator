package controller

import (
	"context"
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
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/pkg/maxscale"
	mxsclient "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
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

	LogMaxScale bool
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=maxscales,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=maxscales/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=maxscales/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;watch;create;patch
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
			name:      "Service",
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

	return ctrl.Result{}, nil
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
		mxs.Spec.Admin.PasswordSecretKeyRef,
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

func (r *MaxScaleReconciler) reconcileAdmin(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	if !maxscale.IsReady() {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// TODO: all Pods in order to support HA
	client, err := r.clientWithPodIndex(ctx, maxscale, 0)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MaxScale client: %v", err)
	}
	err = client.User.Get(ctx, maxscale.Spec.Admin.Username)
	if err == nil {
		return ctrl.Result{}, nil
	}
	if !mxsclient.IsUnautorized(err) && !mxsclient.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("error getting admin user: %v", err)
	}

	// TODO: all Pods in order to support HA
	defaultClient, err := r.defaultClientWithPodIndex(ctx, maxscale, 0)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MaxScale client: %v", err)
	}
	password, err := r.RefResolver.SecretKeyRef(ctx, maxscale.AdminPasswordSecretKeyRef(), maxscale.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting admin password: %v", err)
	}
	if err := defaultClient.User.CreateAdmin(ctx, maxscale.Spec.Admin.Username, password); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating admin user: %v", err)
	}
	if maxscale.Spec.Admin.ShouldDeleteDefaultAdmin() {
		if err := defaultClient.User.DeleteDefaultAdmin(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting default admin: %v", err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileInit(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	if !maxscale.IsReady() {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// TODO: all Pods in order to support HA
	client, err := r.clientWithPodIndex(ctx, maxscale, 0)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MaxScale client: %v", err)
	}

	if err := r.initServers(ctx, maxscale, client); err != nil {
		return ctrl.Result{}, fmt.Errorf("error initializing servers: %v", err)
	}
	if err := r.initServices(ctx, maxscale, client); err != nil {
		return ctrl.Result{}, fmt.Errorf("error initializing services: %v", err)
	}
	if err := r.initMonitor(ctx, maxscale, client); err != nil {
		return ctrl.Result{}, fmt.Errorf("error initializing monitor: %v", err)
	}

	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) initServers(ctx context.Context, mxs *mariadbv1alpha1.MaxScale, client *mxsclient.Client) error {
	servers, err := client.Server.List(ctx)
	if err != nil {
		return fmt.Errorf("error listing servers: %v", err)
	}
	// TODO: handle upgrade
	if len(servers) > 0 {
		return nil
	}

	for _, srv := range mxs.Spec.Servers {
		params := mxsclient.ServerParameters{
			Address:  srv.Address,
			Port:     srv.Port,
			Protocol: srv.Protocol,
		}
		if err := client.Server.Create(ctx, srv.Name, params); err != nil {
			return fmt.Errorf("error creating server: %v", err)
		}
	}
	return nil
}

func (r *MaxScaleReconciler) initServices(ctx context.Context, mxs *mariadbv1alpha1.MaxScale, client *mxsclient.Client) error {
	services, err := client.Service.List(ctx)
	if err != nil {
		return fmt.Errorf("error listing services: %v", err)
	}
	// TODO: handle upgrade
	if len(services) > 0 {
		return nil
	}

	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.ServerPasswordSecretKeyRef, mxs.Namespace)
	if err != nil {
		return fmt.Errorf("error getting server password: %v", err)
	}

	for _, svc := range mxs.Spec.Services {
		svcParams := mxsclient.ServiceParameters{
			User:     mxs.Spec.Auth.ServerUsername,
			Password: password,
			Params:   mxsclient.NewMapParams(svc.Params),
		}
		svcRels := mxsclient.NewServerRelationships(mxs.ServerIDs()...)
		if err := client.Service.Create(ctx, svc.Name, svc.Router, svcParams, svcRels); err != nil {
			return fmt.Errorf("error creating service: %v", err)
		}

		listenerParams := mxsclient.ListenerParameters{
			Port:     svc.Listener.Port,
			Protocol: svc.Listener.Protocol,
			Params:   mxsclient.NewMapParams(svc.Listener.Params),
		}
		listenerRels := mxsclient.NewServiceRelationships(svc.Name)
		if err := client.Listener.Create(ctx, svc.Listener.Name, listenerParams, listenerRels); err != nil {
			return fmt.Errorf("error creating listener: %v", err)
		}
	}
	return nil
}

func (r *MaxScaleReconciler) initMonitor(ctx context.Context, mxs *mariadbv1alpha1.MaxScale, client *mxsclient.Client) error {
	if _, err := client.Monitor.Get(ctx, mxs.Spec.Monitor.Name); err == nil {
		return nil
	}

	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.MonitorPasswordSecretKeyRef, mxs.Namespace)
	if err != nil {
		return fmt.Errorf("error getting monitor password: %v", err)
	}
	params := mxsclient.MonitorParameters{
		User:            mxs.Spec.Auth.MonitorUsername,
		Password:        password,
		MonitorInterval: mxs.Spec.Monitor.Interval,
		Params:          mxsclient.NewMapParams(mxs.Spec.Monitor.Params),
	}
	relations := mxsclient.NewServerRelationships(mxs.ServerIDs()...)
	if err := client.Monitor.Create(ctx, mxs.Spec.Monitor.Name, mxs.Spec.Monitor.Module, params, relations); err != nil {
		return fmt.Errorf("error creating monitor: %v", err)
	}
	return nil
}

func (r *MaxScaleReconciler) defaultClientWithPodIndex(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podIndex int) (*mxsclient.Client, error) {
	opts := []mdbhttp.Option{
		mdbhttp.WithTimeout(10 * time.Second),
	}
	if r.LogMaxScale {
		logger := log.FromContext(ctx).WithName("maxscale-client")
		opts = append(opts, mdbhttp.WithLogger(&logger))
	}
	return mxsclient.NewClientWithDefaultCredentials(mxs.PodAPIUrl(podIndex), opts...)
}

// func (r *MaxScaleReconciler) client(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (*mxsclient.Client, error) {
// 	return r.clientWithAPIUrl(ctx, mxs, mxs.APIUrl())
// }

func (r *MaxScaleReconciler) clientWithPodIndex(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podIndex int) (*mxsclient.Client, error) {
	return r.clientWithAPIUrl(ctx, mxs, mxs.PodAPIUrl(podIndex))
}

func (r *MaxScaleReconciler) clientWithAPIUrl(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	apiUrl string) (*mxsclient.Client, error) {
	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.AdminPasswordSecretKeyRef(), mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting admin password: %v", err)
	}

	opts := []mdbhttp.Option{
		mdbhttp.WithTimeout(10 * time.Second),
		mdbhttp.WithBasicAuth(mxs.Spec.Admin.Username, password),
	}
	if r.LogMaxScale {
		logger := log.FromContext(ctx).WithName("maxscale-client")
		opts = append(opts, mdbhttp.WithLogger(&logger))
	}
	return mxsclient.NewClient(apiUrl, opts...)
}

func (r *MaxScaleReconciler) patcher(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) func(*mariadbv1alpha1.MaxScaleStatus) error {
	return func(mss *mariadbv1alpha1.MaxScaleStatus) error {
		var sts appsv1.StatefulSet
		if err := r.Get(ctx, client.ObjectKeyFromObject(maxscale), &sts); err != nil {
			return err
		}
		maxscale.Status.Replicas = sts.Status.ReadyReplicas

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

// SetupWithManager sets up the controller with the Manager.
func (r *MaxScaleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.MaxScale{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
