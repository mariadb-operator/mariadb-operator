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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builders"
	"github.com/mmontes11/mariadb-operator/pkg/mariadb"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
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

	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariadb, r.RefResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB client: %v", err)
	}
	defer mdbClient.Close()

	if err := r.createExporterCredentials(ctx, mariadb, mdbClient); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating exporter credentials: %v", err)
	}

	return ctrl.Result{}, nil
}

func (r *MonitorMariaDBReconciler) createExporterCredentials(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	mdbClient *mariadb.Client) error {
	key := exporterKey(mariadb)
	privileges := []string{
		"PROCESS",
		"REPLICATION CLIENT",
		"SELECT",
	}
	exists, err := mdbClient.UserExists(ctx, key.Name)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %v", err)
	}
	hasPrivileges, err := mdbClient.UserHasPrivileges(ctx, key.Name, privileges)
	if err != nil {
		return fmt.Errorf("error checking user privileges: %v", err)
	}
	if exists && hasPrivileges {
		return nil
	}

	var user databasev1alpha1.UserMariaDB
	if err := r.createUser(ctx, mariadb); err != nil {
		return fmt.Errorf("error creating user: %v", err)
	}
	err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		err := r.Get(ctx, key, &user)
		if err != nil {
			return false, nil
		}
		return user.IsReady(), nil
	})
	if err != nil {
		return fmt.Errorf("error creating user: %v", err)
	}

	return nil
}

func (r *MonitorMariaDBReconciler) createUser(ctx context.Context, mariadb *databasev1alpha1.MariaDB) error {
	opts := builders.UserOpts{
		Name: exporterKey(mariadb).Name,
		// TODO: generate password secret
		PasswordSecretKeyRef: corev1.SecretKeySelector{},
		MaxUserConnections:   3,
	}
	user := builders.BuildUser(mariadb, opts)
	if err := controllerutil.SetControllerReference(mariadb, user, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to UserMariaDB: %v", err)
	}

	return r.Create(ctx, user)
}

func exporterKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MonitorMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.MonitorMariaDB{}).
		Owns(&databasev1alpha1.UserMariaDB{}).
		Owns(&databasev1alpha1.GrantMariaDB{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Owns(&monitoringv1.PodMonitor{}).
		Complete(r)
}
