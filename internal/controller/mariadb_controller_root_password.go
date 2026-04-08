package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	agentclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/client"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	mdbpod "github.com/mariadb-operator/mariadb-operator/v26/pkg/pod"
	mariadbsql "github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileRootPassword will ensure the root password in the mariadb CR is up to date
// @NOTE: `root@localhost` and `root@%` are modified as both are created when MariaDB is first configured
func (r *MariaDBReconciler) reconcileRootPassword(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsReady() {
		log.FromContext(ctx).V(1).Info("MariaDB not ready. Requeuing root password")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if mariadb.IsRootPasswordEmpty() {
		return ctrl.Result{}, nil
	}

	if result, err := r.shouldReconcileRootPassword(ctx, mariadb); !result.IsZero() || err != nil {
		return result, err
	}

	changePasswordCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return r.reconcileRootPasswordInMariaDB(changePasswordCtx, mariadb)
}

// reconcileRootPasswordInMariaDB will rotate the rootPassword if needed
// If not needed, it will ensure the root password is set to the desired one
func (r *MariaDBReconciler) reconcileRootPasswordInMariaDB(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	rootPassLogger := log.FromContext(ctx).WithName("root-password")

	newRootPassword, err := r.RefResolver.SecretKeyRef(ctx, mariadb.Spec.RootPasswordSecretKeyRef.SecretKeySelector, mariadb.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting root password secret: %v", err)
	}

	internalRootPasswordSecretKey := mariadb.InternalRootPasswordSecretKey()
	var internalRootPasswordSecret corev1.Secret
	err = r.Get(ctx, internalRootPasswordSecretKey, &internalRootPasswordSecret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			rootPassLogger.V(1).Info("Internal root password secret not found, requeuing")
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error getting internal root password secret: %v", err)
	}
	internalRootPassword := internalRootPasswordSecret.Data[mariadb.Spec.RootPasswordSecretKeyRef.Key]

	if newRootPassword == string(internalRootPassword) {
		// Ensure root password is set as expected
		return r.ensureRootPassword(ctx, mariadb)
	}

	rootPassLogger.Info("Root password changed. Updating.")

	sqlClient, err := mariadbsql.NewClientWithMariaDB(
		ctx,
		mariadb,
		r.RefResolver,
		mariadbsql.WithPassword(string(internalRootPassword)), // Using the internal password
		mariadbsql.WithTimeout(5*time.Second),
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating SQL client for root password update: %v", err)
	}
	defer sqlClient.Close()

	// only context cancel error may be returned
	restoreFunc := func(err error) error {
		rootPassLogger.Error(err, "attempting to restore the previous password")

		patchCtx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		return wait.PollUntilContextCancel(patchCtx, time.Second, true, func(ctx context.Context) (done bool, err error) {
			var errBundle *multierror.Error
			errBundle = multierror.Append(
				errBundle,
				sqlClient.AlterUser(
					patchCtx,
					"'root'@'localhost'",
					mariadbsql.WithIdentifiedBy(string(internalRootPassword)),
					mariadbsql.WithMaxUserConnections(10),
				),
			)
			errBundle = multierror.Append(
				errBundle,
				sqlClient.AlterUser(
					patchCtx,
					"'root'@'%'",
					mariadbsql.WithIdentifiedBy(string(internalRootPassword)),
					mariadbsql.WithMaxUserConnections(10),
				),
			)

			if errBundle.ErrorOrNil() == nil {
				return true, nil

			}
			rootPassLogger.Error(errBundle.ErrorOrNil(), "error while restoring root user password")

			return false, nil
		})
	}

	var errBundle *multierror.Error
	errBundle = multierror.Append(
		errBundle,
		sqlClient.AlterUser(
			ctx,
			"'root'@'localhost'",
			mariadbsql.WithIdentifiedBy(newRootPassword),
			mariadbsql.WithMaxUserConnections(10),
		),
	)
	errBundle = multierror.Append(
		errBundle,
		sqlClient.AlterUser(
			ctx,
			"'root'@'%'",
			mariadbsql.WithIdentifiedBy(newRootPassword),
			mariadbsql.WithMaxUserConnections(10),
		),
	)

	if errBundle.ErrorOrNil() != nil {
		return ctrl.Result{}, fmt.Errorf(
			"error updating root user password: %v",
			multierror.Append(errBundle, restoreFunc(errBundle.ErrorOrNil())),
		)
	}

	patch := client.MergeFrom(internalRootPasswordSecret.DeepCopy())
	internalRootPasswordSecret.Data[mariadb.Spec.RootPasswordSecretKeyRef.Key] = []byte(newRootPassword)

	if err := r.Patch(ctx, &internalRootPasswordSecret, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf(
			"error while patching internal root password secret: %v",
			multierror.Append(err, restoreFunc(err)),
		)
	}

	r.Recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonMariaDBRootPasswordChanged,
		mariadbv1alpha1.ActionReconciling, "Root password updated successfully.")
	rootPassLogger.Info("Root password updated successfully. Requeuing...")

	return ctrl.Result{RequeueAfter: time.Second}, nil
}

// ensureRootPasswordInDataPlane updates the root password based on the current value of `RootPasswordSecretKeyRef`
// - Inside the agents for the probes
// - If galera is enabled, updates `wsrep_sst_auth`
// @TODO: When https://github.com/mariadb-operator/mariadb-operator/pull/1621 is merged, we can remove the check for IsHAEnabled
func (r *MariaDBReconciler) ensureRootPasswordInDataPlane(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsHAEnabled() {
		return ctrl.Result{}, nil
	}

	rootPassword, err := r.RefResolver.SecretKeyRef(ctx, mariadb.Spec.RootPasswordSecretKeyRef.SecretKeySelector, mariadb.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting root password secret: %v", err)
	}

	clientSet, err := agentclient.NewClientSet(ctx, mariadb, r.Environment, r.RefResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating agent client set: %v", err)
	}

	pods, err := mdbpod.ListMariaDBPods(ctx, r.Client, mariadb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing MariaDB pods: %v", err)
	}

	g := new(errgroup.Group)
	g.SetLimit(len(pods))
	for i := range pods {
		g.Go(func() error {
			agentClient, err := clientSet.ClientForIndex(i)
			if err != nil {
				return fmt.Errorf("error getting agent client for pod '%s': %v", pods[i].Name, err)
			}
			if err := agentClient.Environment.SetValue(ctx, "MARIADB_ROOT_PASSWORD", rootPassword); err != nil {
				return err
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error setting root password in agent environment: %v", err)
	}

	if mariadb.IsGaleraEnabled() {
		sqlClient, err := mariadbsql.NewClientWithMariaDB(
			ctx,
			mariadb,
			r.RefResolver,
			mariadbsql.WithPassword(rootPassword),
			mariadbsql.WithTimeout(5*time.Second),
		)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating SQL client for root password update: %v", err)
		}
		defer sqlClient.Close()

		if err := sqlClient.ChangeWsrepSSTAuth(ctx, "root", rootPassword); err != nil {
			return ctrl.Result{}, fmt.Errorf("error setting wsrep_sst_auth: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

// shouldReconcileRootPassword checks if a root password reconciliation is safe to happen.
func (r *MariaDBReconciler) shouldReconcileRootPassword(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if mariadb.IsInitializing() || mariadb.IsUpdating() || mariadb.IsRestoringBackup() ||
		mariadb.IsScalingOut() || mariadb.IsRecoveringReplicas() || mariadb.HasGaleraNotReadyCondition() ||
		mariadb.IsSwitchingPrimary() || mariadb.IsReplicationSwitchoverRequired() {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	healthy, err := health.IsStatefulSetHealthy(
		ctx,
		r.Client,
		client.ObjectKeyFromObject(mariadb),
		health.WithDesiredReplicas(mariadb.Spec.Replicas),
		health.WithPort(mariadb.Spec.Port),
		health.WithEndpointPolicy(health.EndpointPolicyAll),
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking MariaDB health: %v", err)
	}
	if !healthy {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	jobList, err := job.ListJobsForMariaDB(ctx, r.Client, mariadb)

	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing jobs: %v", err)
	}

	if job.HasRunningJobs(jobList) {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}
