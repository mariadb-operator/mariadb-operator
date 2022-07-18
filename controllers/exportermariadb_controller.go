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

	"github.com/hashicorp/go-multierror"
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builders"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	"github.com/mmontes11/mariadb-operator/pkg/mariadb"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	exporterPrivileges = []string{
		"SELECT",
		"PROCESS",
		// TODO: check MariaDB version and use 'REPLICATION CLIENT' instead
		// see: https://mariadb.com/kb/en/grant/#binlog-monitor
		"BINLOG MONITOR",
		"SLAVE MONITOR",
	}
	passwordSecretKey = "password"
	dsnSecretKey      = "dsn"
)

// ExporterMariaDBReconciler reconciles a ExporterMariaDB object
type ExporterMariaDBReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=exportermariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=exportermariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=exportermariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ExporterMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var exporter databasev1alpha1.ExporterMariaDB
	if err := r.Get(ctx, req.NamespacedName, &exporter); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var mariaDbErr *multierror.Error
	mariaDb, err := r.RefResolver.GetMariaDB(ctx, exporter.Spec.MariaDBRef, exporter.Namespace)
	if err != nil {
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &exporter, r.ConditionReady.RefResolverPatcher(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	var connErr *multierror.Error
	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariaDb, r.RefResolver)
	if err != nil {
		connErr = multierror.Append(connErr, err)

		err = r.patchStatus(ctx, &exporter, r.ConditionReady.FailedPatcher("Error connecting to MariaDB"))
		connErr = multierror.Append(connErr, err)

		return ctrl.Result{}, fmt.Errorf("error creating MariaDB client: %v", connErr)
	}
	defer mdbClient.Close()

	var userErr *multierror.Error
	user, err := r.createCredentials(ctx, mariaDb, &exporter, mdbClient)
	if err != nil {
		userErr = multierror.Append(userErr, err)

		err = r.patchStatus(ctx, &exporter, r.ConditionReady.FailedPatcher("Error creating exporter credentials"))
		userErr = multierror.Append(userErr, err)

		return ctrl.Result{}, fmt.Errorf("error creating exporter credentials: %v", userErr)
	}

	var deployErr *multierror.Error
	err = r.createDeployment(ctx, mariaDb, &exporter, user)
	deployErr = multierror.Append(deployErr, err)

	err = r.patchStatus(ctx, &exporter, r.ConditionReady.PatcherWithError(err))
	deployErr = multierror.Append(deployErr, err)

	if err := deployErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating exporter Deployment: %v", deployErr)
	}
	return ctrl.Result{}, nil
}

func (r *ExporterMariaDBReconciler) createCredentials(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	exporter *databasev1alpha1.ExporterMariaDB, mdbClient *mariadb.Client) (*databasev1alpha1.UserMariaDB, error) {
	key := exporterKey(mariadb)
	if err := r.createUser(ctx, mariadb, exporter); err != nil {
		return nil, fmt.Errorf("error creating UserMariaDB: %v", err)
	}

	var user databasev1alpha1.UserMariaDB
	err := wait.PollImmediateWithContext(ctx, 1*time.Second, 10*time.Second, func(ctx context.Context) (bool, error) {
		if err := r.Get(ctx, key, &user); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return true, err
		}
		return user.IsReady(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for UserMariaDB to be ready: %v", err)
	}

	if err := r.createGrant(ctx, mariadb, exporter, &user); err != nil {
		return nil, fmt.Errorf("error creating GrantMariaDB: %v", err)
	}

	var grant databasev1alpha1.GrantMariaDB
	err = wait.PollImmediateWithContext(ctx, 1*time.Second, 10*time.Second, func(ctx context.Context) (bool, error) {
		if err := r.Get(ctx, key, &grant); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return true, err
		}
		return grant.IsReady(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for GrantMariaDB to be ready: %v", err)
	}

	return &user, nil
}

func (r *ExporterMariaDBReconciler) createDeployment(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	exporter *databasev1alpha1.ExporterMariaDB, user *databasev1alpha1.UserMariaDB) error {
	key := exporterKey(mariadb)
	var existingDeploy v1.Deployment
	if err := r.Get(ctx, key, &existingDeploy); err == nil {
		return nil
	}

	dsnSecretKeySelector, err := r.createDsnSecret(ctx, mariadb, exporter, user)
	if err != nil {
		return fmt.Errorf("error creating DSN Secret: %v", err)
	}
	deploy, err := builders.BuildExporterDeployment(mariadb, exporter, key, dsnSecretKeySelector)
	if err != nil {
		return fmt.Errorf("error building exporter Deployment: %v", err)
	}
	if err := controllerutil.SetControllerReference(exporter, deploy, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to exporter Deployment: %v", err)
	}

	if err := r.Create(ctx, deploy); err != nil {
		return fmt.Errorf("error creating exporter Deployment: %v", err)
	}
	return nil
}

func (r *ExporterMariaDBReconciler) createUser(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	exporter *databasev1alpha1.ExporterMariaDB) error {
	key := exporterKey(mariadb)
	var existingUser databasev1alpha1.UserMariaDB
	if err := r.Get(ctx, key, &existingUser); err == nil {
		return nil
	}

	secretKeySelector, err := r.createPasswordSecret(ctx, mariadb, exporter)
	if err != nil {
		return fmt.Errorf("error creating user password Secret: %v", err)
	}

	opts := builders.UserOpts{
		Key:                  key,
		PasswordSecretKeyRef: *secretKeySelector,
		MaxUserConnections:   3,
	}
	user := builders.BuildUserMariaDB(mariadb, opts)
	if err := controllerutil.SetControllerReference(exporter, user, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to UserMariaDB: %v", err)
	}

	if err := r.Create(ctx, user); err != nil {
		return fmt.Errorf("error creating UserMariaDB: %v", err)
	}
	return nil
}

func (r *ExporterMariaDBReconciler) createGrant(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	exporter *databasev1alpha1.ExporterMariaDB, user *databasev1alpha1.UserMariaDB) error {
	key := exporterKey(mariadb)
	var grantMariaDB databasev1alpha1.GrantMariaDB
	if err := r.Get(ctx, key, &grantMariaDB); err == nil {
		return nil
	}

	opts := builders.GrantOpts{
		Key:         key,
		Privileges:  exporterPrivileges,
		Database:    "*",
		Table:       "*",
		Username:    user.Name,
		GrantOption: false,
	}
	grant := builders.BuildGrantMariaDB(mariadb, opts)
	if err := controllerutil.SetControllerReference(exporter, grant, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to GrantMariaDB: %v", err)
	}

	return r.Create(ctx, grant)
}

func (r *ExporterMariaDBReconciler) createPasswordSecret(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	exporter *databasev1alpha1.ExporterMariaDB) (*corev1.SecretKeySelector, error) {
	password, err := password.Generate(16, 4, 0, false, false)
	if err != nil {
		return nil, fmt.Errorf("error generating password: %v", err)
	}

	opts := builders.SecretOpts{
		Key: passwordKey(mariadb),
		Data: map[string][]byte{
			passwordSecretKey: []byte(password),
		},
	}
	secret := builders.BuildSecret(mariadb, opts)
	if err := controllerutil.SetControllerReference(exporter, secret, r.Scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to password Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("error creating password Secret: %v", err)
	}

	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
		Key: passwordSecretKey,
	}, nil
}

func (r *ExporterMariaDBReconciler) createDsnSecret(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	exporter *databasev1alpha1.ExporterMariaDB, user *databasev1alpha1.UserMariaDB) (*corev1.SecretKeySelector, error) {
	password, err := r.RefResolver.ReadSecretKeyRef(ctx, user.Spec.PasswordSecretKeyRef, mariadb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting password: %v", err)
	}
	mdbOpts := mariadbclient.Opts{
		Username: user.Name,
		Password: password,
		Host:     mariadb.Name,
		Port:     mariadb.Spec.Port,
	}
	dsn, err := mariadbclient.BuildDSN(mdbOpts)
	if err != nil {
		return nil, fmt.Errorf("error building DSN: %v", err)
	}

	secretOpts := builders.SecretOpts{
		Key: dsnKey(mariadb),
		Data: map[string][]byte{
			dsnSecretKey: []byte(dsn),
		},
	}
	secret := builders.BuildSecret(mariadb, secretOpts)
	if err := controllerutil.SetControllerReference(exporter, secret, r.Scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to DSN Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("error creating DSN Secret: %v", err)
	}
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
		Key: dsnSecretKey,
	}, nil
}

func (r *ExporterMariaDBReconciler) patchStatus(ctx context.Context, exporter *databasev1alpha1.ExporterMariaDB,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(exporter.DeepCopy())
	patcher(&exporter.Status)

	if err := r.Client.Status().Patch(ctx, exporter, patch); err != nil {
		return fmt.Errorf("error patching ExporterMariaDB status: %v", err)
	}
	return nil
}

func exporterKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func passwordKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter-password", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func dsnKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter-dsn", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExporterMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.ExporterMariaDB{}).
		Owns(&databasev1alpha1.UserMariaDB{}).
		Owns(&databasev1alpha1.GrantMariaDB{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
