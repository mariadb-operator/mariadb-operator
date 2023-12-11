package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	ctrlresources "github.com/mariadb-operator/mariadb-operator/controller/resource"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/endpoints"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/galera"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MariaDBReconciler reconciles a MariaDB object
type MariaDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Builder        *builder.Builder
	RefResolver    *refresolver.RefResolver
	ConditionReady *condition.Ready
	Environment    *environment.Environment

	ServiceMonitorReconciler bool

	ConfigMapReconciler *configmap.ConfigMapReconciler
	SecretReconciler    *secret.SecretReconciler
	ServiceReconciler   *service.ServiceReconciler
	EndpointsReconciler *endpoints.EndpointsReconciler
	RBACReconciler      *rbac.RBACReconciler

	ReplicationReconciler *replication.ReplicationReconciler
	GaleraReconciler      *galera.GaleraReconciler
}

type reconcilePhase struct {
	Name      string
	Reconcile func(context.Context, *mariadbv1alpha1.MariaDB) error
}

type patcher func(*mariadbv1alpha1.MariaDBStatus) error

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=mariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=mariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=mariadbs/finalizers,verbs=update
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=restores;connections,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=create;patch;get;list;watch
//+kubebuilder:rbac:groups="",resources=endpoints/restricted,verbs=create;patch;get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterrolebindings,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create
//+kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var mariaDb mariadbv1alpha1.MariaDB
	if err := r.Get(ctx, req.NamespacedName, &mariaDb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	phases := []reconcilePhase{
		{
			Name:      "Spec",
			Reconcile: r.setSpecDefaults,
		},
		{
			Name:      "Status",
			Reconcile: r.setStatusDefaults,
		},
		{
			Name:      "Secret",
			Reconcile: r.reconcileSecret,
		},
		{
			Name:      "ConfigMap",
			Reconcile: r.reconcileConfigMap,
		},
		{
			Name:      "RBAC",
			Reconcile: r.RBACReconciler.Reconcile,
		},
		{
			Name:      "StatefulSet",
			Reconcile: r.reconcileStatefulSet,
		},
		{
			Name:      "PodDisruptionBudget",
			Reconcile: r.reconcilePodDisruptionBudget,
		},
		{
			Name:      "Service",
			Reconcile: r.reconcileService,
		},
		{
			Name:      "Connection",
			Reconcile: r.reconcileConnection,
		},
		{
			Name:      "Replication",
			Reconcile: r.ReplicationReconciler.Reconcile,
		},
		{
			Name:      "Galera",
			Reconcile: r.GaleraReconciler.Reconcile,
		},
		{
			Name:      "Restore",
			Reconcile: r.reconcileRestore,
		},
	}
	if r.ServiceMonitorReconciler {
		phases = append(phases, reconcilePhase{
			Name:      "ServiceMonitor",
			Reconcile: r.reconcileServiceMonitor,
		})
	}

	for _, p := range phases {
		if err := p.Reconcile(ctx, &mariaDb); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			var errBundle *multierror.Error
			errBundle = multierror.Append(errBundle, err)

			msg := fmt.Sprintf("Error reconciling %s: %v", p.Name, err)
			patchErr := r.patchStatus(ctx, &mariaDb, func(s *mariadbv1alpha1.MariaDBStatus) error {
				patcher := r.ConditionReady.PatcherFailed(msg)
				patcher(s)
				return nil
			})
			if apierrors.IsNotFound(patchErr) {
				errBundle = multierror.Append(errBundle, patchErr)
			}

			if err := errBundle.ErrorOrNil(); err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling %s: %v", p.Name, err)
			}
		}
	}

	if err := r.Get(ctx, req.NamespacedName, &mariaDb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if err := r.patchStatus(ctx, &mariaDb, r.patcher(ctx, &mariaDb)); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileSecret(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	secretKeyRef := mariadb.Spec.RootPasswordSecretKeyRef
	key := types.NamespacedName{
		Name:      secretKeyRef.Name,
		Namespace: mariadb.Namespace,
	}
	_, err := r.SecretReconciler.ReconcileRandomPassword(ctx, key, secretKeyRef.Key, mariadb)
	return err
}

func (r *MariaDBReconciler) reconcileConfigMap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.MyCnf != nil && mariadb.Spec.MyCnfConfigMapKeyRef != nil {
		configMapKeyRef := *mariadb.Spec.MyCnfConfigMapKeyRef
		req := configmap.ReconcileRequest{
			Mariadb: mariadb,
			Owner:   mariadb,
			Key: types.NamespacedName{
				Name:      configMapKeyRef.Name,
				Namespace: mariadb.Namespace,
			},
			Data: map[string]string{
				configMapKeyRef.Key: *mariadb.Spec.MyCnf,
			},
		}
		return r.ConfigMapReconciler.Reconcile(ctx, &req)
	}
	return nil
}

func (r *MariaDBReconciler) reconcileStatefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(mariadb)
	var dsn *corev1.SecretKeySelector
	if mariadb.AreMetricsEnabled() {
		var err error
		dsn, err = r.reconcileMetricsCredentials(ctx, mariadb)
		if err != nil {
			return fmt.Errorf("error creating metrics credentials: %v", err)
		}
	}

	desiredSts, err := r.Builder.BuildStatefulSet(mariadb, key, dsn)
	if err != nil {
		return fmt.Errorf("error building StatefulSet: %v", err)
	}

	var existingSts appsv1.StatefulSet
	if err := r.Get(ctx, key, &existingSts); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting StatefulSet: %v", err)
		}
		if err := r.Create(ctx, desiredSts); err != nil {
			return fmt.Errorf("error creating StatefulSet: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingSts.DeepCopy())
	existingSts.Spec.Template = desiredSts.Spec.Template
	existingSts.Spec.Replicas = desiredSts.Spec.Replicas
	return r.Patch(ctx, &existingSts, patch)
}

func (r *MariaDBReconciler) reconcilePodDisruptionBudget(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.IsHAEnabled() && mariadb.Spec.PodDisruptionBudget == nil {
		return r.reconcileHighAvailabilityPDB(ctx, mariadb)
	}
	return r.reconcileDefaultPDB(ctx, mariadb)
}

func (r *MariaDBReconciler) reconcileService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.IsHAEnabled() {
		if err := r.reconcileInternalService(ctx, mariadb); err != nil {
			return err
		}
		if err := r.reconcilePrimaryService(ctx, mariadb); err != nil {
			return err
		}
		if err := r.reconcileSecondaryService(ctx, mariadb); err != nil {
			return err
		}
	}
	return r.reconcileDefaultService(ctx, mariadb)
}

func (r *MariaDBReconciler) reconcileConnection(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.IsHAEnabled() {
		if mariadb.Spec.PrimaryConnection != nil {
			key := ctrlresources.PrimaryConnectioneKey(mariadb)
			serviceName := ctrlresources.PrimaryServiceKey(mariadb).Name
			connTpl := mariadb.Spec.PrimaryConnection
			connTpl.ServiceName = &serviceName

			if err := r.reconcileConnectionTemplate(ctx, key, connTpl, mariadb); err != nil {
				return err
			}
		}
		if mariadb.Spec.SecondaryConnection != nil {
			key := ctrlresources.SecondaryConnectioneKey(mariadb)
			serviceName := ctrlresources.SecondaryServiceKey(mariadb).Name
			connTpl := mariadb.Spec.SecondaryConnection
			connTpl.ServiceName = &serviceName

			if err := r.reconcileConnectionTemplate(ctx, key, connTpl, mariadb); err != nil {
				return err
			}
		}
	}
	return r.reconcileDefaultConnection(ctx, mariadb)
}

func (r *MariaDBReconciler) reconcileRestore(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.BootstrapFrom == nil {
		return nil
	}
	if mariadb.HasRestoredBackup() {
		return nil
	}
	if mariadb.IsRestoringBackup() {
		key := restoreKey(mariadb)
		var existingRestore mariadbv1alpha1.Restore
		if err := r.Get(ctx, key, &existingRestore); err != nil {
			return err
		}
		return r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			if existingRestore.IsComplete() {
				condition.SetRestoredBackup(status)
			} else {
				condition.SetRestoringBackup(status)
			}
			return nil
		})
	}

	healthy, err := health.IsMariaDBHealthy(ctx, r.Client, mariadb, health.EndpointPolicyAll)
	if err != nil {
		return fmt.Errorf("error checking MariaDB health: %v", err)
	}
	if !healthy {
		return nil
	}

	key := restoreKey(mariadb)
	var existingRestore mariadbv1alpha1.Restore
	if err := r.Get(ctx, key, &existingRestore); err == nil {
		return nil
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetRestoringBackup(status)
		return nil
	}); err != nil {
		return fmt.Errorf("error patching status: %v", err)
	}

	restore, err := r.Builder.BuildRestore(mariadb, key)
	if err != nil {
		return fmt.Errorf("error building restore: %v", err)
	}
	return r.Create(ctx, restore)
}

func (r *MariaDBReconciler) reconcileServiceMonitor(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Metrics == nil {
		return nil
	}

	key := client.ObjectKeyFromObject(mariadb)
	var existingServiceMontor monitoringv1.ServiceMonitor
	if err := r.Get(ctx, key, &existingServiceMontor); err == nil {
		return nil
	}

	serviceMonitor, err := r.Builder.BuildServiceMonitor(mariadb, key)
	if err != nil {
		return fmt.Errorf("error building Service Monitor: %v", err)
	}
	return r.Create(ctx, serviceMonitor)
}

func (r *MariaDBReconciler) reconcileDefaultPDB(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.PodDisruptionBudget == nil {
		return nil
	}

	key := client.ObjectKeyFromObject(mariadb)
	var existingPDB policyv1.PodDisruptionBudget
	if err := r.Get(ctx, key, &existingPDB); err == nil {
		return nil
	}

	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			Build()
	opts := builder.PodDisruptionBudgetOpts{
		MariaDB:        mariadb,
		Key:            key,
		MinAvailable:   mariadb.Spec.PodDisruptionBudget.MinAvailable,
		MaxUnavailable: mariadb.Spec.PodDisruptionBudget.MaxUnavailable,
		SelectorLabels: selectorLabels,
	}
	pdb, err := r.Builder.BuildPodDisruptionBudget(&opts, mariadb)
	if err != nil {
		return fmt.Errorf("error building PodDisruptionBudget: %v", err)
	}
	return r.Create(ctx, pdb)
}

func (r *MariaDBReconciler) reconcileHighAvailabilityPDB(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(mariadb)
	var existingPDB policyv1.PodDisruptionBudget
	if err := r.Get(ctx, key, &existingPDB); err == nil {
		return nil
	}

	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			Build()
	minAvailable := intstr.FromString("50%")
	opts := builder.PodDisruptionBudgetOpts{
		MariaDB:        mariadb,
		Key:            key,
		MinAvailable:   &minAvailable,
		SelectorLabels: selectorLabels,
	}
	pdb, err := r.Builder.BuildPodDisruptionBudget(&opts, mariadb)
	if err != nil {
		return fmt.Errorf("error building PodDisruptionBudget: %v", err)
	}
	return r.Create(ctx, pdb)
}

func (r *MariaDBReconciler) reconcileDefaultService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(mariadb)
	opts := builder.ServiceOpts{
		Ports: []corev1.ServicePort{
			{
				Name: builder.MariaDbPortName,
				Port: mariadb.Spec.Port,
			},
		},
	}
	if mariadb.Spec.Service != nil {
		opts.ServiceTemplate = *mariadb.Spec.Service
	}
	if mariadb.AreMetricsEnabled() {
		opts.Ports = append(opts.Ports, corev1.ServicePort{
			Name: builder.MetricsContainerName,
			Port: mariadb.Spec.Metrics.Exporter.Port,
		})
		// To match ServiceMonitor selector
		opts.Labels =
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mariadb).
				WithLabels(opts.Labels).
				Build()
	}
	desiredSvc, err := r.Builder.BuildService(mariadb, key, opts)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MariaDBReconciler) reconcileInternalService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := ctrlresources.InternalServiceKey(mariadb)
	ports := []corev1.ServicePort{
		{
			Name: builder.MariaDbPortName,
			Port: mariadb.Spec.Port,
		},
	}
	if mariadb.Galera().Enabled {
		ports = append(ports, []corev1.ServicePort{
			{
				Name: galeraresources.GaleraClusterPortName,
				Port: galeraresources.GaleraClusterPort,
			},
			{
				Name: galeraresources.GaleraISTPortName,
				Port: galeraresources.GaleraISTPort,
			},
			{
				Name: galeraresources.GaleraSSTPortName,
				Port: galeraresources.GaleraSSTPort,
			},
			{
				Name: galeraresources.AgentPortName,
				Port: *mariadb.Galera().Agent.Port,
			},
		}...)
	}

	opts := builder.ServiceOpts{
		Ports:    ports,
		Headless: true,
	}
	desiredSvc, err := r.Builder.BuildService(mariadb, key, opts)
	if err != nil {
		return fmt.Errorf("error building internal Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MariaDBReconciler) reconcilePrimaryService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	key := ctrlresources.PrimaryServiceKey(mariadb)
	serviceLabels :=
		labels.NewLabelsBuilder().
			WithMariaDBSelectorLabels(mariadb).
			WithStatefulSetPod(mariadb, *mariadb.Status.CurrentPrimaryPodIndex).
			Build()
	opts := builder.ServiceOpts{
		Selectorlabels: serviceLabels,
		Ports: []corev1.ServicePort{
			{
				Name: builder.MariaDbPortName,
				Port: mariadb.Spec.Port,
			},
		},
	}
	if mariadb.Spec.PrimaryService != nil {
		opts.ServiceTemplate = *mariadb.Spec.PrimaryService
	}
	desiredSvc, err := r.Builder.BuildService(mariadb, key, opts)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MariaDBReconciler) reconcileSecondaryService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := ctrlresources.SecondaryServiceKey(mariadb)
	opts := builder.ServiceOpts{
		ExcludeSelectorLabels: true,
		Ports: []corev1.ServicePort{
			{
				Name: builder.MariaDbPortName,
				Port: mariadb.Spec.Port,
			},
		},
	}
	if mariadb.Spec.SecondaryService != nil {
		opts.ServiceTemplate = *mariadb.Spec.SecondaryService
	}
	desiredSvc, err := r.Builder.BuildService(mariadb, key, opts)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}
	if err := r.ServiceReconciler.Reconcile(ctx, desiredSvc); err != nil {
		return err
	}
	if err := r.EndpointsReconciler.Reconcile(ctx, ctrlresources.SecondaryServiceKey(mariadb), mariadb); err != nil {
		if errors.Is(err, endpoints.ErrNoAddressesAvailable) {
			log.FromContext(ctx).V(1).Info("No addresses available for secondary Endpoints")
			return nil
		}
		return err
	}
	return nil
}

func (r *MariaDBReconciler) reconcileDefaultConnection(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Connection == nil || mariadb.Spec.Username == nil || mariadb.Spec.PasswordSecretKeyRef == nil ||
		!mariadb.IsReady() {
		return nil
	}
	key := client.ObjectKeyFromObject(mariadb)
	var existingConn mariadbv1alpha1.Connection
	if err := r.Get(ctx, key, &existingConn); err == nil {
		return nil
	}

	connOpts := builder.ConnectionOpts{
		MariaDB:              mariadb,
		Key:                  key,
		Username:             *mariadb.Spec.Username,
		PasswordSecretKeyRef: *mariadb.Spec.PasswordSecretKeyRef,
		Database:             mariadb.Spec.Database,
		Template:             mariadb.Spec.Connection,
	}
	conn, err := r.Builder.BuildConnection(connOpts, mariadb)
	if err != nil {
		return fmt.Errorf("error building Connection: %v", err)
	}
	return r.Create(ctx, conn)
}

func (r *MariaDBReconciler) reconcileConnectionTemplate(ctx context.Context, key types.NamespacedName,
	connTpl *mariadbv1alpha1.ConnectionTemplate, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Username == nil || mariadb.Spec.PasswordSecretKeyRef == nil || !mariadb.IsReady() {
		return nil
	}
	var existingConn mariadbv1alpha1.Connection
	if err := r.Get(ctx, key, &existingConn); err == nil {
		return nil
	}

	connOpts := builder.ConnectionOpts{
		MariaDB:              mariadb,
		Key:                  key,
		Username:             *mariadb.Spec.Username,
		PasswordSecretKeyRef: *mariadb.Spec.PasswordSecretKeyRef,
		Database:             mariadb.Spec.Database,
		Template:             connTpl,
	}
	conn, err := r.Builder.BuildConnection(connOpts, mariadb)
	if err != nil {
		return fmt.Errorf("erro building Connection: %v", err)
	}
	return r.Create(ctx, conn)
}

func (r *MariaDBReconciler) setSpecDefaults(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	return r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) {
		mdb.SetDefaults(r.Environment)
	})
}

func (r *MariaDBReconciler) setStatusDefaults(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Status.CurrentPrimaryPodIndex != nil && mariadb.Status.CurrentPrimary != nil {
		return nil
	}
	return r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.FillWithDefaults(mariadb)
		return nil
	})
}

func (r *MariaDBReconciler) patcher(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) patcher {
	return func(s *mariadbv1alpha1.MariaDBStatus) error {
		if mariadb.IsRestoringBackup() ||
			mariadb.IsConfiguringReplication() || mariadb.IsSwitchingPrimary() ||
			mariadb.HasGaleraNotReadyCondition() {
			return nil
		}

		var sts appsv1.StatefulSet
		if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
			return err
		}
		condition.SetReadyWithStatefulSet(&mariadb.Status, &sts)
		return nil
	}
}

func (r *MariaDBReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher patcher) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	if err := patcher(&mariadb.Status); err != nil {
		return fmt.Errorf("error patching MariaDB status object: %v", err)
	}
	return r.Status().Patch(ctx, mariadb, patch)
}

func (r *MariaDBReconciler) patch(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDB)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(mariadb)
	return r.Patch(ctx, mariadb, patch)
}

func restoreKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("restore-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.MariaDB{}).
		Owns(&mariadbv1alpha1.Connection{}).
		Owns(&mariadbv1alpha1.Restore{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Event{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&rbacv1.ClusterRoleBinding{})
	if r.ServiceMonitorReconciler {
		builder = builder.Owns(&monitoringv1.ServiceMonitor{})
	}
	return builder.Complete(r)
}
