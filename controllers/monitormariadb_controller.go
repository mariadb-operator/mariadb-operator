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

	"github.com/sethvargo/go-password/password"
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

var (
	monitorPrivileges = []string{
		"PROCESS",
		// TODO: check MariaDB version and use 'REPLICATION CLIENT' instead
		// see: https://mariadb.com/kb/en/grant/#binlog-monitor
		"BINLOG MONITOR",
		"SELECT",
	}
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

	if err := r.createExporterCredentials(ctx, mariadb, &monitor, mdbClient); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating exporter credentials: %v", err)
	}

	return ctrl.Result{}, nil
}

func (r *MonitorMariaDBReconciler) createExporterCredentials(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB, mdbClient *mariadb.Client) error {
	key := exporterKey(mariadb)
	exists, err := mdbClient.UserExists(ctx, key.Name)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %v", err)
	}
	hasPrivileges, err := mdbClient.UserHasPrivileges(ctx, key.Name, monitorPrivileges)
	if err != nil {
		return fmt.Errorf("error checking user privileges: %v", err)
	}
	if exists && hasPrivileges {
		return nil
	}

	if err := r.createUser(ctx, mariadb, monitor); err != nil {
		return fmt.Errorf("error creating UserMariaDB: %v", err)
	}
	var user databasev1alpha1.UserMariaDB
	err = wait.PollImmediateWithContext(ctx, 1*time.Second, 30*time.Second, func(ctx context.Context) (bool, error) {
		if r.Get(ctx, key, &user) != nil {
			return false, nil
		}
		return user.IsReady(), nil
	})
	if err != nil {
		return fmt.Errorf("error creating UserMariaDB: %v", err)
	}

	if err := r.createGrant(ctx, mariadb, monitor, &user); err != nil {
		return fmt.Errorf("error creating GrantMariaDB: %v", err)
	}
	var grant databasev1alpha1.GrantMariaDB
	err = wait.PollImmediateWithContext(ctx, 1*time.Second, 30*time.Second, func(ctx context.Context) (bool, error) {
		if r.Get(ctx, key, &grant) != nil {
			return false, nil
		}
		return grant.IsReady(), nil
	})
	if err != nil {
		return fmt.Errorf("error creating GrantMariaDB: %v", err)
	}

	return nil
}

func (r *MonitorMariaDBReconciler) createUser(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB) error {
	key := exporterKey(mariadb).Name
	secretKeySelector, err := r.createPassword(ctx, mariadb, monitor)
	if err != nil {
		return fmt.Errorf("error creating user password: %v", err)
	}

	opts := builders.UserOpts{
		Name:                 key,
		PasswordSecretKeyRef: *secretKeySelector,
		MaxUserConnections:   3,
	}
	user := builders.BuildUser(mariadb, opts)
	if err := controllerutil.SetControllerReference(monitor, user, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to UserMariaDB: %v", err)
	}

	return r.Create(ctx, user)
}

func (r *MonitorMariaDBReconciler) createGrant(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB, user *databasev1alpha1.UserMariaDB) error {
	opts := builders.GrantOpts{
		Name:        exporterKey(mariadb).Name,
		Privileges:  monitorPrivileges,
		Database:    "*",
		Table:       "*",
		Username:    user.Name,
		GrantOption: false,
	}
	grant := builders.BuildGrant(mariadb, opts)
	if err := controllerutil.SetControllerReference(monitor, grant, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to GrantMariaDB: %v", err)
	}

	return r.Create(ctx, grant)
}

func (r *MonitorMariaDBReconciler) createPassword(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB) (*corev1.SecretKeySelector, error) {
	password, err := password.Generate(64, 10, 10, false, false)
	if err != nil {
		return nil, fmt.Errorf("error generating passowrd: %v", err)
	}

	secretKey := "password"
	opts := builders.SecretOpts{
		Name: exporterKey(mariadb).Name,
		Data: map[string][]byte{
			secretKey: []byte(password),
		},
	}
	secret := builders.BuildSecret(mariadb, opts)
	if err := controllerutil.SetControllerReference(monitor, secret, r.Scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Secret: %v", err)
	}
	if err := r.Client.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("error creating Secret on API server: %v", err)
	}

	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: exporterKey(mariadb).Name,
		},
		Key: secretKey,
	}, nil
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
