package controller

import (
	"context"
	"fmt"

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
	"github.com/mariadb-operator/mariadb-operator/pkg/maxscale"
)

// MaxScaleReconciler reconciles a MaxScale object
type MaxScaleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	Builder        *builder.Builder
	ConditionReady *condition.Ready
	Environment    *environment.Environment

	SecretReconciler      *secret.SecretReconciler
	StatefulSetReconciler *statefulset.StatefulSetReconciler
	ServiceReconciler     *service.ServiceReconciler
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
	key := types.NamespacedName{
		Name:      secretKeyRef.Name,
		Namespace: mxs.Namespace,
	}
	config, err := maxscale.Config(mxs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MaxScale config: %v", err)
	}

	req := secret.ReconcileRequest{
		Owner: mxs,
		Key:   key,
		Data: map[string][]byte{
			secretKeyRef.Key: config,
		},
	}
	return ctrl.Result{}, r.SecretReconciler.Reconcile(ctx, &req)
}

func (r *MaxScaleReconciler) reconcileStatefulSet(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale) (ctrl.Result, error) {
	key := client.ObjectKeyFromObject(maxscale)
	desiredSts, err := r.Builder.BuildMaxscaleStatefulSet(maxscale, key)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building StatefulSet: %v", err)
	}
	return ctrl.Result{}, r.StatefulSetReconciler.Reconcile(ctx, desiredSts)
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
			Port: 8989,
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
