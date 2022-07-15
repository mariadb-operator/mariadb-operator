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
	"time"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builders"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// MonitorMariaDBReconciler reconciles a MonitorMariaDB object
type MonitorMariaDBReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	RefResolver *refresolver.RefResolver
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=monitormariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=monitormariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=monitormariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MonitorMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var monitor databasev1alpha1.MonitorMariaDB
	if err := r.Get(ctx, req.NamespacedName, &monitor); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariadb, err := r.RefResolver.GetMariaDB(ctx, monitor.Spec.MariaDBRef, monitor.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", err)
	}

	if err = r.createExporter(ctx, mariadb, &monitor); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating exporter: %v", err)
	}

	err = r.createPodMonitor(ctx, mariadb, &monitor)
	if patchErr := r.patchStatus(ctx, &monitor, conditions.NewConditionReadyPatcher(err)); patchErr != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MonitorMariaDB status: %v", err)
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating PodMonitor: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MonitorMariaDBReconciler) createExporter(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB) error {
	key := objectKey(mariadb)
	var existingExporter databasev1alpha1.ExporterMariaDB
	if err := r.Get(ctx, key, &existingExporter); err == nil {
		return nil
	}

	exporter := builders.BuildExporter(mariadb, &monitor.Spec.Exporter, key)
	if err := controllerutil.SetControllerReference(monitor, exporter, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to ExporterMariaDB: %v", err)
	}
	if err := r.Create(ctx, exporter); err != nil {
		return fmt.Errorf("error creating ExporterMariaDB in API server: %v", err)
	}

	err := wait.PollImmediateWithContext(ctx, 1*time.Second, 30*time.Second, func(ctx context.Context) (bool, error) {
		var exporter databasev1alpha1.ExporterMariaDB
		if r.Get(ctx, objectKey(mariadb), &exporter) != nil {
			return false, nil
		}
		return exporter.IsReady(), nil
	})
	if err != nil {
		return fmt.Errorf("error creating ExporterMariaDB: %v", err)
	}

	return nil
}

func (r *MonitorMariaDBReconciler) createPodMonitor(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB) error {
	key := objectKey(mariadb)
	var existingPodMonitor monitoringv1.PodMonitor
	if err := r.Get(ctx, key, &existingPodMonitor); err == nil {
		return nil
	}

	podMonitor := builders.BuildPodMonitor(mariadb, monitor, key)
	if err := controllerutil.SetControllerReference(monitor, podMonitor, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to PodMonitor: %v", err)
	}

	if err := r.Create(ctx, podMonitor); err != nil {
		return fmt.Errorf("error creating PodMonitor in API server: %v", err)
	}
	return nil
}

func (r *MonitorMariaDBReconciler) patchStatus(ctx context.Context, monitor *databasev1alpha1.MonitorMariaDB,
	patcher conditions.ConditionPatcher) error {
	patch := client.MergeFrom(monitor.DeepCopy())
	patcher(&monitor.Status)
	return r.Client.Status().Patch(ctx, monitor, patch)
}

func objectKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      mariadb.Name,
		Namespace: mariadb.Namespace,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MonitorMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.MonitorMariaDB{}).
		Owns(&databasev1alpha1.ExporterMariaDB{}).
		Owns(&monitoringv1.PodMonitor{}).
		Complete(r)
}
