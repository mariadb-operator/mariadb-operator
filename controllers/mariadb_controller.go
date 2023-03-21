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
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MariaDBReconciler reconciles a MariaDB object
type MariaDBReconciler struct {
	client.Client
	Scheme                   *runtime.Scheme
	Builder                  *builder.Builder
	RefResolver              *refresolver.RefResolver
	ConditionReady           *conditions.Ready
	ConfigMapReconciler      *configmap.ConfigMapReconciler
	ServiceMonitorReconciler bool
}

type MariaDBReconcilePhase struct {
	Resource  string
	Reconcile func(context.Context, *mariadbv1alpha1.MariaDB, types.NamespacedName) error
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=mariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=mariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=mariadbs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=restores,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var mariaDb mariadbv1alpha1.MariaDB
	if err := r.Get(ctx, req.NamespacedName, &mariaDb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	phases := []MariaDBReconcilePhase{
		{
			Resource:  "ConfigMap",
			Reconcile: r.reconcileConfigMap,
		},
		{
			Resource:  "StatefulSet",
			Reconcile: r.reconcileStatefulSet,
		},
		{
			Resource:  "Service",
			Reconcile: r.reconcileService,
		},
		{
			Resource:  "Connection",
			Reconcile: r.reconcileConnection,
		},
	}
	if r.ServiceMonitorReconciler {
		phases = append(phases, MariaDBReconcilePhase{
			Resource:  "ServiceMonitor",
			Reconcile: r.reconcileServiceMonitor,
		})
	}
	phases = append(phases, MariaDBReconcilePhase{
		Resource:  "Restore",
		Reconcile: r.reconcileBootstrapRestore,
	})

	for _, p := range phases {
		if err := p.Reconcile(ctx, &mariaDb, req.NamespacedName); err != nil {
			var errBundle *multierror.Error
			errBundle = multierror.Append(errBundle, err)

			err = r.patchStatus(ctx, &mariaDb, r.ConditionReady.FailedPatcher(fmt.Sprintf("Error creating %s", p.Resource)))
			errBundle = multierror.Append(errBundle, err)

			return ctrl.Result{}, fmt.Errorf("error creating %s: %v", p.Resource, errBundle)
		}
	}

	patcher, err := r.patcher(ctx, &mariaDb, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, fmt.Errorf("error getting patcher for MariaDB: %v", err)
	}
	if err = r.patchStatus(ctx, &mariaDb, patcher); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %s", err)
	}

	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileConfigMap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	mariaDbKey types.NamespacedName) error {

	if r.ConfigMapReconciler.NoopReconcile(mariadb) {
		return nil
	}

	key := configMapMariaDBKey(mariadb)
	if err := r.ConfigMapReconciler.Reconcile(ctx, mariadb, key); err != nil {
		return fmt.Errorf("error reconciling ConfigMap: %v", err)
	}

	if err := r.patch(ctx, mariadb, func(md *mariadbv1alpha1.MariaDB) {
		mariadb.Spec.MyCnfConfigMapKeyRef = &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: key.Name,
			},
			Key: r.ConfigMapReconciler.ConfigMapKey,
		}
	}); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) reconcileStatefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName) error {

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

	if err := r.Patch(ctx, &existingSts, patch); err != nil {
		return fmt.Errorf("error patching StatefulSet: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) reconcileService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName) error {

	desiredSvc, err := r.Builder.BuildService(mariadb, key)
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

	if err := r.Patch(ctx, &existingSvc, patch); err != nil {
		return fmt.Errorf("error patching Service: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) reconcileServiceMonitor(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName) error {
	if mariadb.Spec.Metrics == nil {
		return nil
	}
	var existingServiceMontor monitoringv1.ServiceMonitor
	if err := r.Get(ctx, key, &existingServiceMontor); err == nil {
		return nil
	}

	serviceMonitor, err := r.Builder.BuildServiceMonitor(mariadb, key)
	if err != nil {
		return fmt.Errorf("error building Service Monitor: %v", err)
	}

	if err := r.Create(ctx, serviceMonitor); err != nil {
		return fmt.Errorf("error creating Service Monitor: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) reconcileBootstrapRestore(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	mariaDbKey types.NamespacedName) error {
	if mariadb.Spec.BootstrapFrom == nil || !mariadb.IsReady() || mariadb.IsBootstrapped() {
		return nil
	}
	key := bootstrapRestoreKey(mariadb)
	var existingRestore mariadbv1alpha1.Restore
	if err := r.Get(ctx, key, &existingRestore); err == nil {
		return nil
	}

	restore, err := r.Builder.BuildRestore(
		mariadb,
		mariadb.Spec.BootstrapFrom,
		key,
	)
	if err != nil {
		return fmt.Errorf("error building restore: %v", err)
	}

	if err := r.Create(ctx, restore); err != nil {
		return fmt.Errorf("error creating bootstrapping restore Job: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) reconcileConnection(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	mariaDbKey types.NamespacedName) error {
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
		Key: key,
		MariaDBRef: mariadbv1alpha1.MariaDBRef{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: mariadb.Name,
			},
			WaitForIt: true,
		},
		Username:             *mariadb.Spec.Username,
		PasswordSecretKeyRef: *mariadb.Spec.PasswordSecretKeyRef,
		Database:             mariadb.Spec.Database,
		Template:             mariadb.Spec.Connection,
	}
	conn, err := r.Builder.BuildConnection(connOpts, mariadb)
	if err != nil {
		return fmt.Errorf("erro building Connection: %v", err)
	}

	if err := r.Create(ctx, conn); err != nil {
		return fmt.Errorf("error creating Connection: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) patcher(ctx context.Context, mariaDb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName) (conditions.Patcher, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, key, &sts); err != nil {
		return nil, err
	}

	var restore mariadbv1alpha1.Restore
	var restoreExists bool
	if err := r.Get(ctx, bootstrapRestoreKey(mariaDb), &restore); err != nil {
		if apierrors.IsNotFound(err) {
			restoreExists = false
		} else {
			return nil, err
		}
	} else {
		restoreExists = true
	}

	return func(c conditions.Conditioner) {
		if sts.Status.Replicas == 0 || sts.Status.ReadyReplicas != sts.Status.Replicas {
			c.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonStatefulSetNotReady,
				Message: "Not ready",
			})
			return
		}
		if !restoreExists {
			c.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionTrue,
				Reason:  mariadbv1alpha1.ConditionReasonStatefulSetReady,
				Message: "Running",
			})
			return
		}
		if mariaDb.IsBootstrapped() {
			c.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionTrue,
				Reason:  mariadbv1alpha1.ConditionReasonStatefulSetReady,
				Message: "Running",
			})
			return
		}
		if restore.IsComplete() {
			c.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeBootstrapped,
				Status:  metav1.ConditionTrue,
				Reason:  mariadbv1alpha1.ConditionReasonRestoreComplete,
				Message: "Ready",
			})
			c.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionTrue,
				Reason:  mariadbv1alpha1.ConditionReasonRestoreComplete,
				Message: "Running",
			})
		} else {
			c.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonRestoreNotComplete,
				Message: "Restoring backup",
			})
			c.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeBootstrapped,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonRestoreNotComplete,
				Message: "Not ready",
			})
		}
	}, nil
}

func (r *MariaDBReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)

	if err := r.Client.Status().Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) patch(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDB)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(mariadb)

	if err := r.Client.Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}
	return nil
}

func bootstrapRestoreKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("bootstrap-restore-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func connectionKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("connection-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func configMapMariaDBKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("config-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.MariaDB{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&mariadbv1alpha1.Connection{}).
		Owns(&mariadbv1alpha1.Restore{})
	if r.ServiceMonitorReconciler {
		builder = builder.Owns(&monitoringv1.ServiceMonitor{})
	}
	return builder.Complete(r)
}
