/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	myCnfConfigMapKey = "my.cnf"
)

// MariaDBReconciler reconciles a MariaDB object
type MariaDBReconciler struct {
	client.Client
	Scheme                   *runtime.Scheme
	Builder                  *builder.Builder
	RefResolver              *refresolver.RefResolver
	ConditionReady           *conditions.Ready
	ConfigMapReconciler      *configmap.ConfigMapReconciler
	SecretReconciler         *secret.SecretReconciler
	ReplicationReconciler    *replication.ReplicationReconciler
	ServiceMonitorReconciler bool
}

type reconcilePhase struct {
	Resource  string
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
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=list;watch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=list;watch;create;patch
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
			Resource:  "ConfigMap",
			Reconcile: r.reconcileConfigMap,
		},
		{
			Resource:  "StatefulSet",
			Reconcile: r.reconcileStatefulSet,
		},
		{
			Resource:  "PodDisruptionBudget",
			Reconcile: r.reconcilePodDisruptionBudget,
		},
		{
			Resource:  "Service",
			Reconcile: r.reconcileService,
		},
		{
			Resource:  "Connection",
			Reconcile: r.reconcileConnection,
		},
		{
			Resource:  "Replication",
			Reconcile: r.ReplicationReconciler.Reconcile,
		},
		{
			Resource:  "Restore",
			Reconcile: r.reconcileRestore,
		},
	}
	if r.ServiceMonitorReconciler {
		phases = append(phases, reconcilePhase{
			Resource:  "ServiceMonitor",
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

			msg := fmt.Sprintf("Error reconciling %s: %v", p.Resource, err)
			patchErr := r.patchStatus(ctx, &mariaDb, func(s *mariadbv1alpha1.MariaDBStatus) error {
				patcher := r.ConditionReady.PatcherFailed(msg)
				patcher(s)
				return nil
			})
			if apierrors.IsNotFound(patchErr) {
				errBundle = multierror.Append(errBundle, patchErr)
			}

			if err := errBundle.ErrorOrNil(); err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling %s: %v", p.Resource, err)
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

func (r *MariaDBReconciler) reconcileConfigMap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.MyCnf == nil && mariadb.Spec.MyCnfConfigMapKeyRef == nil {
		return nil
	}
	key := configMapMariaDBKey(mariadb)
	if mariadb.Spec.MyCnf != nil && mariadb.Spec.MyCnfConfigMapKeyRef == nil {
		req := configmap.ReconcileRequest{
			Mariadb: mariadb,
			Owner:   mariadb,
			Key:     key,
			Data: map[string]string{
				myCnfConfigMapKey: *mariadb.Spec.MyCnf,
			},
		}
		if err := r.ConfigMapReconciler.Reconcile(ctx, &req); err != nil {
			return fmt.Errorf("error reconciling ConfigMap: %v", err)
		}
	}
	if mariadb.Spec.MyCnfConfigMapKeyRef != nil {
		return nil
	}

	return r.patch(ctx, mariadb, func(md *mariadbv1alpha1.MariaDB) {
		mariadb.Spec.MyCnfConfigMapKeyRef = &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: key.Name,
			},
			Key: myCnfConfigMapKey,
		}
	})
}

func (r *MariaDBReconciler) reconcileStatefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(mariadb)
	var dsn *corev1.SecretKeySelector
	if mariadb.Spec.Metrics != nil {
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
	if mariadb.Spec.PodDisruptionBudget == nil {
		return nil
	}

	key := podDisruptionBudgetKey(mariadb)
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

func (r *MariaDBReconciler) reconcileService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(mariadb)
	serviceLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			Build()
	opts := builder.ServiceOpts{
		Selectorlabels: serviceLabels,
	}
	if mariadb.Spec.Service != nil {
		opts.Type = mariadb.Spec.Service.Type
		opts.Annotations = mariadb.Spec.Service.Annotations
	}
	desiredSvc, err := r.Builder.BuildService(mariadb, key, opts)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}

	var existingSvc corev1.Service
	if err := r.Get(ctx, key, &existingSvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting Service: %v", err)
		}
		if err := r.Create(ctx, desiredSvc); err != nil {
			return fmt.Errorf("error creating Service: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingSvc.DeepCopy())
	existingSvc.Spec.Ports = desiredSvc.Spec.Ports
	existingSvc.Spec.Type = desiredSvc.Spec.Type
	existingSvc.Annotations = desiredSvc.Annotations
	existingSvc.Labels = desiredSvc.Labels
	return r.Patch(ctx, &existingSvc, patch)
}

func (r *MariaDBReconciler) reconcileConnection(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Connection == nil || mariadb.Spec.Username == nil || mariadb.Spec.PasswordSecretKeyRef == nil ||
		!mariadb.IsReady() {
		return nil
	}
	key := connectionKey(mariadb)
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
		return fmt.Errorf("erro building Connection: %v", err)
	}
	return r.Create(ctx, conn)
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
				conditions.SetRestoredBackup(status)
			} else {
				conditions.SetRestoringBackup(status)
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
		conditions.SetRestoringBackup(status)
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

func (r *MariaDBReconciler) patcher(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) patcher {
	return func(s *mariadbv1alpha1.MariaDBStatus) error {
		if mariadb.IsRestoringBackup() || mariadb.IsConfiguringReplication() || mariadb.IsSwitchingPrimary() {
			return nil
		}
		if mariadb.Spec.Replication == nil {
			s.UpdateCurrentPrimary(mariadb, 0)
		}

		var sts appsv1.StatefulSet
		if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
			return err
		}
		conditions.SetReadyWithStatefulSet(&mariadb.Status, &sts)
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

func configMapMariaDBKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("config-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func podDisruptionBudgetKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      mariadb.Name,
		Namespace: mariadb.Namespace,
	}
}

func connectionKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      mariadb.Name,
		Namespace: mariadb.Namespace,
	}
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
		Owns(&appsv1.StatefulSet{}).
		Owns(&policyv1.PodDisruptionBudget{})
	if r.ServiceMonitorReconciler {
		builder = builder.Owns(&monitoringv1.ServiceMonitor{})
	}
	return builder.Complete(r)
}
