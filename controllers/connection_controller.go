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
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	errConnHealthCheck = errors.New("error checking connection health")
)

// ConnectionReconciler reconciles a Connection object
type ConnectionReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Builder        *builder.Builder
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=connections,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=connections/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=connections/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConnectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var conn v1alpha1.Connection
	if err := r.Get(ctx, req.NamespacedName, &conn); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariaDb, refErr := r.RefResolver.MariaDB(ctx, &conn.Spec.MariaDBRef, conn.Namespace)
	if refErr != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, refErr)

		patchErr := r.patchStatus(ctx, &conn, r.ConditionReady.RefResolverPatcher(refErr, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, patchErr)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	if conn.Spec.MariaDBRef.WaitForIt && !mariaDb.IsReady() {
		if err := r.patchStatus(ctx, &conn, r.ConditionReady.FailedPatcher("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Connection: %v", err)
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	if err := r.init(ctx, &conn); err != nil {
		var initErr *multierror.Error
		initErr = multierror.Append(initErr, err)

		patchErr := r.patchStatus(
			ctx,
			&conn,
			r.ConditionReady.FailedPatcher(fmt.Sprintf("error initializing connection: %v", err)),
		)
		initErr = multierror.Append(initErr, patchErr)

		return ctrl.Result{}, fmt.Errorf("error initializing connection: %v", initErr)
	}

	var secretErr *multierror.Error
	err := r.reconcileSecret(ctx, &conn, mariaDb)
	if errors.Is(err, errConnHealthCheck) {
		return ctrl.Result{RequeueAfter: r.retryInterval(&conn)}, nil
	}
	secretErr = multierror.Append(secretErr, err)

	patchErr := r.patchStatus(ctx, &conn, r.ConditionReady.HealthyPatcher(err))
	secretErr = multierror.Append(secretErr, patchErr)

	if err := secretErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating Secret: %v", err)
	}
	return ctrl.Result{RequeueAfter: r.healthCheckInterval(&conn)}, nil
}

func (r *ConnectionReconciler) init(ctx context.Context, conn *mariadbv1alpha1.Connection) error {
	if conn.IsInit() {
		return nil
	}
	patcher := func(c *mariadbv1alpha1.Connection) {
		c.Init()
	}
	if err := r.patch(ctx, conn, patcher); err != nil {
		return fmt.Errorf("error patching restore: %v", err)
	}
	return nil
}

func (r *ConnectionReconciler) reconcileSecret(ctx context.Context, conn *mariadbv1alpha1.Connection,
	mdb *mariadbv1alpha1.MariaDB) error {
	logger := log.FromContext(ctx)
	key := types.NamespacedName{
		Name:      conn.SecretName(),
		Namespace: conn.Namespace,
	}

	var existingSecret corev1.Secret
	if err := r.Get(ctx, key, &existingSecret); err == nil {
		if err := r.healthCheck(ctx, conn, &existingSecret); err != nil {
			logger.Error(err, "error checking connection health")
			return errConnHealthCheck
		}
		return nil
	}

	password, err := r.RefResolver.SecretKeyRef(ctx, conn.Spec.PasswordSecretKeyRef, conn.Namespace)
	if err != nil {
		return fmt.Errorf("error getting password for connection DSN: %v", err)
	}

	var host string
	if conn.Spec.PodIndex != nil {
		host = statefulset.PodFQDN(mdb.ObjectMeta, *conn.Spec.PodIndex)
	} else {
		host = statefulset.ServiceFQDN(mdb.ObjectMeta)
	}
	mdbOpts := mariadb.Opts{
		Username: conn.Spec.Username,
		Password: password,
		Host:     host,
		Port:     mdb.Spec.Port,
		Params:   conn.Spec.Params,
	}
	if conn.Spec.Database != nil {
		mdbOpts.Database = *conn.Spec.Database
	}
	dsn, err := mariadb.BuildDSN(mdbOpts)
	if err != nil {
		return fmt.Errorf("error building DSN: %v", err)
	}

	secretOpts := builder.SecretOpts{
		Key: key,
		Data: map[string][]byte{
			conn.SecretKey(): []byte(dsn),
		},
		Labels:      conn.Spec.SecretTemplate.Labels,
		Annotations: conn.Spec.SecretTemplate.Annotations,
	}
	secret, err := r.Builder.BuildSecret(secretOpts, conn)
	if err != nil {
		return fmt.Errorf("error building Secret: %v", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("error creating Secret: %v", err)
	}
	return nil
}

func (r *ConnectionReconciler) healthCheck(ctx context.Context, conn *mariadbv1alpha1.Connection, secret *corev1.Secret) error {
	logger := log.FromContext(ctx)
	secretKey := conn.SecretKey()
	dsn, ok := secret.Data[secretKey]
	if !ok {
		return fmt.Errorf("connection secret '%s' key not found", secretKey)
	}

	logger.V(1).Info("checking connection health")
	if _, err := mariadb.Connect(string(dsn)); err != nil {
		var connErr *multierror.Error
		connErr = multierror.Append(connErr, err)

		patchErr := r.patchStatus(
			ctx,
			conn,
			r.ConditionReady.HealthyPatcher(fmt.Errorf("failed to connect: %v", err)),
		)
		return multierror.Append(connErr, patchErr)
	}

	if err := r.patchStatus(ctx, conn, r.ConditionReady.HealthyPatcher(nil)); err != nil {
		return fmt.Errorf("error patching connection status: %v", err)
	}
	return nil
}

func (r *ConnectionReconciler) retryInterval(conn *mariadbv1alpha1.Connection) time.Duration {
	if conn.Spec.HealthCheck != nil && conn.Spec.HealthCheck.RetryInterval != nil {
		return (*conn.Spec.HealthCheck.RetryInterval).Duration
	}
	return 3 * time.Second
}

func (r *ConnectionReconciler) healthCheckInterval(conn *mariadbv1alpha1.Connection) time.Duration {
	if conn.Spec.HealthCheck != nil && conn.Spec.HealthCheck.Interval != nil {
		return (*conn.Spec.HealthCheck.Interval).Duration
	}
	return 0
}

func (r *ConnectionReconciler) patchStatus(ctx context.Context, conn *mariadbv1alpha1.Connection,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(conn.DeepCopy())
	patcher(&conn.Status)

	if err := r.Client.Status().Patch(ctx, conn, patch); err != nil {
		return fmt.Errorf("error patching connection status: %v", err)
	}
	return nil
}

func (r *ConnectionReconciler) patch(ctx context.Context, conn *mariadbv1alpha1.Connection,
	patcher func(*mariadbv1alpha1.Connection)) error {
	patch := client.MergeFrom(conn.DeepCopy())
	patcher(conn)

	if err := r.Client.Patch(ctx, conn, patch); err != nil {
		return fmt.Errorf("error patching connection: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConnectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.Connection{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
