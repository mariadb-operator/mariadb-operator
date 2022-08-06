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
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builder"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
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
	Scheme         *runtime.Scheme
	Builder        *builder.Builder
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=mariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=mariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=mariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var mariaDb databasev1alpha1.MariaDB
	if err := r.Get(ctx, req.NamespacedName, &mariaDb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.createStatefulSet(ctx, &mariaDb, req.NamespacedName); err != nil {
		var stsErr *multierror.Error
		stsErr = multierror.Append(stsErr, err)

		err = r.patchStatus(ctx, &mariaDb, r.ConditionReady.FailedPatcher("Error creating StatefulSet"))
		stsErr = multierror.Append(stsErr, err)

		return ctrl.Result{}, fmt.Errorf("error creating StatefulSet: %v", stsErr)
	}

	if err := r.createService(ctx, &mariaDb, req.NamespacedName); err != nil {
		var svcErr *multierror.Error
		svcErr = multierror.Append(svcErr, err)

		err = r.patchStatus(ctx, &mariaDb, r.ConditionReady.FailedPatcher("Error creating Service"))
		svcErr = multierror.Append(svcErr, err)

		return ctrl.Result{}, fmt.Errorf("error creating Service: %v", svcErr)
	}

	if err := r.createServiceMonitor(ctx, &mariaDb, req.NamespacedName); err != nil {
		var monitorErr *multierror.Error
		monitorErr = multierror.Append(monitorErr, err)

		err = r.patchStatus(ctx, &mariaDb, r.ConditionReady.FailedPatcher("Error creatin ServiceMonitor"))
		monitorErr = multierror.Append(monitorErr, err)

		return ctrl.Result{}, fmt.Errorf("error creating ServiceMonitor: %v", monitorErr)
	}

	if err := r.bootstrapFromBackup(ctx, &mariaDb); err != nil {
		var restoreErr *multierror.Error
		restoreErr = multierror.Append(restoreErr, err)

		err = r.patchStatus(ctx, &mariaDb, r.ConditionReady.FailedPatcher("Error creating bootstrapping RestoreMariaDB"))
		restoreErr = multierror.Append(restoreErr, err)

		return ctrl.Result{}, fmt.Errorf("error creating bootstrapping RestoreMariaDB: %v", restoreErr)
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

func (r *MariaDBReconciler) createStatefulSet(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	key types.NamespacedName) error {
	var existingSts appsv1.StatefulSet
	if err := r.Get(ctx, key, &existingSts); err == nil {
		return nil
	}

	var dsn *corev1.SecretKeySelector
	if mariadb.Spec.Metrics != nil {
		var err error
		dsn, err = r.createMetricsCredentials(ctx, mariadb)
		if err != nil {
			return fmt.Errorf("error creating metrics credentials: %v", err)
		}
	}

	sts, err := r.Builder.BuildStatefulSet(mariadb, key, dsn)
	if err != nil {
		return fmt.Errorf("error building StatefulSet: %v", err)
	}

	if err := r.Create(ctx, sts); err != nil {
		return fmt.Errorf("error creating StatefulSet: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) createService(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	key types.NamespacedName) error {
	var existingSvc corev1.Service
	if err := r.Get(ctx, key, &existingSvc); err == nil {
		return nil
	}

	svc, err := r.Builder.BuildService(mariadb, key)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}

	if err := r.Create(ctx, svc); err != nil {
		return fmt.Errorf("error creating Service: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) createServiceMonitor(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
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

func (r *MariaDBReconciler) patcher(ctx context.Context, mariaDb *databasev1alpha1.MariaDB,
	key types.NamespacedName) (conditions.Patcher, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, key, &sts); err != nil {
		return nil, err
	}

	var restore databasev1alpha1.RestoreMariaDB
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
				Type:    databasev1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  databasev1alpha1.ConditionReasonStatefulSetNotReady,
				Message: "Not ready",
			})
			return
		}
		if !restoreExists {
			c.SetCondition(metav1.Condition{
				Type:    databasev1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionTrue,
				Reason:  databasev1alpha1.ConditionReasonStatefulSetReady,
				Message: "Running",
			})
			return
		}

		if mariaDb.IsBootstrapped() {
			c.SetCondition(metav1.Condition{
				Type:    databasev1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionTrue,
				Reason:  databasev1alpha1.ConditionReasonStatefulSetReady,
				Message: "Running",
			})
			return
		}
		if restore.IsComplete() {
			c.SetCondition(metav1.Condition{
				Type:    databasev1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionTrue,
				Reason:  databasev1alpha1.ConditionReasonRestoreComplete,
				Message: "Running",
			})
			c.SetCondition(metav1.Condition{
				Type:    databasev1alpha1.ConditionTypeBootstrapped,
				Status:  metav1.ConditionTrue,
				Reason:  databasev1alpha1.ConditionReasonRestoreComplete,
				Message: "Ready",
			})
		} else {
			c.SetCondition(metav1.Condition{
				Type:    databasev1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  databasev1alpha1.ConditionReasonRestoreNotComplete,
				Message: "Restoring backup",
			})
			c.SetCondition(metav1.Condition{
				Type:    databasev1alpha1.ConditionTypeBootstrapped,
				Status:  metav1.ConditionFalse,
				Reason:  databasev1alpha1.ConditionReasonRestoreNotComplete,
				Message: "Not ready",
			})
		}
	}, nil
}

func (r *MariaDBReconciler) patchStatus(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)

	if err := r.Client.Status().Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.MariaDB{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&databasev1alpha1.RestoreMariaDB{}).
		Complete(r)
}
