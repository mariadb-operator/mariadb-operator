package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ExternalMariaDBReconciler reconciles a ExternalMariaDB object
type ExternalMariaDBReconciler struct {
	client.Client
	Recorder       record.EventRecorder
	Builder        *builder.Builder
	RefResolver    *refresolver.RefResolver
	ConditionReady *condition.Ready
	Environment    *environment.OperatorEnv
}

type patcherExternalMariaDB func(*mariadbv1alpha1.ExternalMariaDBStatus) error

type reconcilePhaseExternalMariaDB struct {
	Name      string
	Reconcile func(context.Context, *mariadbv1alpha1.ExternalMariaDB) (ctrl.Result, error)
}

//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=externalmariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=externalmariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=externalmariadbs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ExternalMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var external_mariadb mariadbv1alpha1.ExternalMariaDB
	if err := r.Get(ctx, req.NamespacedName, &external_mariadb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	phases := []reconcilePhaseExternalMariaDB{
		{
			Name:      "Spec",
			Reconcile: r.setSpecDefaults,
		},
		{
			Name:      "Status",
			Reconcile: r.reconcileStatus,
		},
		{
			Name:      "Connection",
			Reconcile: r.reconcileConnection,
		},
	}

	for _, p := range phases {
		result, err := p.Reconcile(ctx, &external_mariadb)
		if err != nil {

			log.FromContext(ctx).V(1).Info("Phase name", "name", p.Name)

			var errBundle *multierror.Error
			errBundle = multierror.Append(errBundle, err)

			msg := fmt.Sprintf("Error reconciling %s: %v", p.Name, err)
			patchErr := r.patchStatus(ctx, &external_mariadb, func(s *mariadbv1alpha1.ExternalMariaDBStatus) error {
				patcher := r.ConditionReady.PatcherFailed(msg)
				patcher(s)
				return nil
			})
			if !apierrors.IsNotFound(patchErr) {
				errBundle = multierror.Append(errBundle, patchErr)
			}

			if err := errBundle.ErrorOrNil(); err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling %s: %v", p.Name, err)
			}
		}
		if !result.IsZero() {
			patchErr := r.patchStatus(ctx, &external_mariadb, func(s *mariadbv1alpha1.ExternalMariaDBStatus) error {
				patcher := r.ConditionReady.PatcherHealthy(err)
				patcher(s)
				return nil
			})

			if patchErr != nil {
				return result, err
			}
			return result, err
		}

	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *ExternalMariaDBReconciler) setSpecDefaults(ctx context.Context,
	external_mariadb *mariadbv1alpha1.ExternalMariaDB) (ctrl.Result, error) {
	return ctrl.Result{}, r.patch(ctx, external_mariadb, func(emdb *mariadbv1alpha1.ExternalMariaDB) error {
		return emdb.SetDefaults(r.Environment)
	})
}

func (r *ExternalMariaDBReconciler) reconcileConnection(ctx context.Context,
	extMariaDB *mariadbv1alpha1.ExternalMariaDB) (ctrl.Result, error) {

	if extMariaDB.Spec.Connection == nil {
		return ctrl.Result{}, nil
	}
	if !extMariaDB.IsReady() {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	key := client.ObjectKeyFromObject(extMariaDB)
	var existingConn mariadbv1alpha1.Connection
	if err := r.Get(ctx, key, &existingConn); err == nil {
		return ctrl.Result{}, nil
	}

	connOpts := builder.ConnectionOpts{
		Metadata:             extMariaDB.Spec.InheritMetadata,
		ExternalMariaDB:      extMariaDB,
		Key:                  key,
		Username:             *extMariaDB.Spec.Username,
		PasswordSecretKeyRef: extMariaDB.Spec.PasswordSecretKeyRef,
		Template:             extMariaDB.Spec.Connection,
	}
	conn, err := r.Builder.BuildConnection(connOpts, extMariaDB)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building Connection: %v", err)
	}
	return ctrl.Result{}, r.Create(ctx, conn)

}

func (r *ExternalMariaDBReconciler) reconcileStatus(ctx context.Context,
	extMariaDB *mariadbv1alpha1.ExternalMariaDB) (ctrl.Result, error) {

	if !extMariaDB.IsReady() {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	client, err := sqlClient.NewClientWithMariaDB(ctx, extMariaDB, r.RefResolver)
	if err != nil {
		return ctrl.Result{RequeueAfter: 3 * time.Second}, fmt.Errorf("error connecting to MariaDB: %v", err)
	}
	defer client.Close()

	rawVersion, err := client.SystemVariable(ctx, "version")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get MariaDB version: %v", err)
	}
	versionParts := strings.Split(rawVersion, "-")

	var version string
	if len(versionParts) > 0 {
		version = versionParts[0]
	} else {
		msg := "MariaDB version could not be inferred"
		log.FromContext(ctx).Error(errors.New(msg), msg, "version", rawVersion)
	}

	isGaleraEnabled, err := client.IsSystemVariableEnabled(ctx, "wsrep_on")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to determine if Galera cluster is enable on that cluster: %v", err)
	}

	return ctrl.Result{}, r.patchStatus(ctx, extMariaDB, func(status *mariadbv1alpha1.ExternalMariaDBStatus) error {
		status.SetVersion(version)
		status.Version = version
		status.IsGaleraEnabled = isGaleraEnabled
		condition.SetReadyHealthy(&extMariaDB.Status)
		return nil
	})

}

func (r *ExternalMariaDBReconciler) patchStatus(ctx context.Context, external_mariadb *mariadbv1alpha1.ExternalMariaDB,
	patcher patcherExternalMariaDB) error {
	patch := client.MergeFrom(external_mariadb.DeepCopy())
	if err := patcher(&external_mariadb.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, external_mariadb, patch)
}

func (r *ExternalMariaDBReconciler) patch(ctx context.Context,
	external_mariadb *mariadbv1alpha1.ExternalMariaDB, patcher func(*mariadbv1alpha1.ExternalMariaDB) error) error {
	patch := client.MergeFrom(external_mariadb.DeepCopy())
	if err := patcher(external_mariadb); err != nil {
		return err
	}

	return r.Patch(ctx, external_mariadb, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.ExternalMariaDB{}).
		Owns(&mariadbv1alpha1.Connection{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func NewExternalMariaDBReconciler(client client.Client, refResolver *refresolver.RefResolver, conditionReady *condition.Ready,
	builder *builder.Builder) *ExternalMariaDBReconciler {
	return &ExternalMariaDBReconciler{
		Client:         client,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
		Builder:        builder,
	}
}
