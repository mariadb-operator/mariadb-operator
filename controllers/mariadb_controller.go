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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builders"
)

// MariaDBReconciler reconciles a MariaDB object
type MariaDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=mariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=mariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=mariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var mariadb databasev1alpha1.MariaDB
	if err := r.Get(ctx, req.NamespacedName, &mariadb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, req.NamespacedName, &sts); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting StatefulSet: %v", err)
		}

		if err := r.createStatefulSet(ctx, &mariadb); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating StatefulSet: %v", err)
		}
	}

	var svc corev1.Service
	if err := r.Get(ctx, req.NamespacedName, &svc); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting Service: %v", err)
		}

		if err := r.createService(ctx, &mariadb); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating Service: %v", err)
		}
	}

	var restore databasev1alpha1.RestoreMariaDB
	restoreExists := true
	if mariadb.Spec.BootstrapFromBackup != nil {
		if err := r.Get(ctx, bootstrapRestoreKey(&mariadb), &restore); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("error getting bootstrap Restore: %v", err)
			}
			restoreExists = false
		}
	}

	if err := r.patchMariaDBStatus(ctx, &mariadb, &sts, &restore); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	if !restoreExists && shouldBootstrapFromBackup(&mariadb) {
		if err := r.bootstrapFromBackup(ctx, &mariadb); err != nil {
			return ctrl.Result{}, fmt.Errorf("error bootstrapping MariaDB from backup: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) createStatefulSet(ctx context.Context, mariadb *databasev1alpha1.MariaDB) error {
	sts, err := builders.BuildStatefulSet(mariadb)
	if err != nil {
		return fmt.Errorf("error building StatefulSet %v", err)
	}
	if err := controllerutil.SetControllerReference(mariadb, sts, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to StatefulSet: %v", err)
	}

	if err := r.Create(ctx, sts); err != nil {
		return fmt.Errorf("error creating StatefulSet on API server: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) createService(ctx context.Context, mariadb *databasev1alpha1.MariaDB) error {
	svc := builders.BuildService(mariadb)
	if err := controllerutil.SetControllerReference(mariadb, svc, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	if err := r.Create(ctx, svc); err != nil {
		return fmt.Errorf("error creating Service on API server: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) patchMariaDBStatus(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	sts *appsv1.StatefulSet, restore *databasev1alpha1.RestoreMariaDB) error {
	patch := client.MergeFrom(mariadb.DeepCopy())

	if sts.Status.Replicas == 0 || sts.Status.ReadyReplicas < sts.Status.Replicas {
		mariadb.Status.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  databasev1alpha1.ConditionReasonStatefulSetNotReady,
			Message: "Not ready",
		})
	} else if sts.Status.ReadyReplicas == sts.Status.Replicas {
		setMariaDBStatusReady(mariadb, restore)
	} else {
		mariadb.Status.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  databasev1alpha1.ConditionReasonStatefulUnknownState,
			Message: "Unknown state",
		})
	}

	return r.Client.Status().Patch(ctx, mariadb, patch)
}

func (r *MariaDBReconciler) bootstrapFromBackup(ctx context.Context, mariadb *databasev1alpha1.MariaDB) error {
	restoreKey := bootstrapRestoreKey(mariadb)
	restore := databasev1alpha1.RestoreMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreKey.Name,
			Namespace: restoreKey.Namespace,
		},
		Spec: databasev1alpha1.RestoreMariaDBSpec{
			MariaDBRef: corev1.LocalObjectReference{
				Name: mariadb.Name,
			},
			BackupRef: mariadb.Spec.BootstrapFromBackup.BackupRef,
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, &restore, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to bootstrap Restore: %v", err)
	}

	if err := r.Create(ctx, &restore); err != nil {
		return fmt.Errorf("error creating bootstrap Restore job: %v", err)
	}

	return nil
}

func setMariaDBStatusReady(mariadb *databasev1alpha1.MariaDB, restore *databasev1alpha1.RestoreMariaDB) {
	if mariadb.Spec.BootstrapFromBackup == nil || mariadb.IsBootstrapped() {
		mariadb.Status.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonStatefulSetReady,
			Message: "Running",
		})
		return
	}

	if restore.IsComplete() {
		mariadb.Status.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonRestoreComplete,
			Message: "Running",
		})
		mariadb.Status.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeBootstrapped,
			Status:  metav1.ConditionTrue,
			Reason:  databasev1alpha1.ConditionReasonRestoreComplete,
			Message: "Ready",
		})
	} else {
		mariadb.Status.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  databasev1alpha1.ConditionReasonRestoreNotComplete,
			Message: "Restoring backup",
		})
		mariadb.Status.AddCondition(metav1.Condition{
			Type:    databasev1alpha1.ConditionTypeBootstrapped,
			Status:  metav1.ConditionFalse,
			Reason:  databasev1alpha1.ConditionReasonRestoreNotComplete,
			Message: "Not ready",
		})
	}
}

func shouldBootstrapFromBackup(mariadb *databasev1alpha1.MariaDB) bool {
	return mariadb.Spec.BootstrapFromBackup != nil && !mariadb.IsBootstrapped()
}

func bootstrapRestoreKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("bootstrap-restore-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.MariaDB{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&databasev1alpha1.RestoreMariaDB{}).
		Complete(r)
}
