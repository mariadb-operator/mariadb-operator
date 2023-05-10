package sql

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SqlReconciler struct {
	Client         client.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready

	WrappedReconciler WrappedReconciler
	Finalizer         Finalizer
}

func NewSqlReconciler(client client.Client, cr *conditions.Ready, wr WrappedReconciler, f Finalizer) Reconciler {
	return &SqlReconciler{
		Client:            client,
		RefResolver:       refresolver.New(client),
		ConditionReady:    cr,
		WrappedReconciler: wr,
		Finalizer:         f,
	}
}

func (tr *SqlReconciler) Reconcile(ctx context.Context, resource Resource) (ctrl.Result, error) {
	if resource.IsBeingDeleted() {
		if err := tr.Finalizer.Finalize(ctx, resource); err != nil {
			return ctrl.Result{}, fmt.Errorf("error finalizing %s: %v", resource.GetName(), err)
		}
		return ctrl.Result{}, nil
	}

	if err := tr.Finalizer.AddFinalizer(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error adding finalizer to %s: %v", resource.GetName(), err)
	}

	mariadb, err := tr.RefResolver.MariaDB(ctx, resource.MariaDBRef(), resource.GetNamespace())
	if err != nil {
		var mariadbErr *multierror.Error
		mariadbErr = multierror.Append(mariadbErr, err)

		err = tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.PatcherRefResolver(err, mariadb))
		mariadbErr = multierror.Append(mariadbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariadbErr)
	}

	if err := waitForMariaDB(ctx, tr.Client, resource, mariadb); err != nil {
		var errBundle *multierror.Error
		errBundle = multierror.Append(errBundle, err)

		if err := tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.PatcherWithError(err)); err != nil {
			errBundle = multierror.Append(errBundle, err)
		}

		if err := errBundle.ErrorOrNil(); err != nil {
			return ctrl.Result{}, fmt.Errorf("error waiting for MariaDB: %v", err)
		}
	}

	// TODO: connection pooling. See https://github.com/mariadb-operator/mariadb-operator/issues/7.
	mdbClient, err := mariadbclient.NewRootClient(ctx, mariadb, tr.RefResolver)
	if err != nil {
		var errBundle *multierror.Error
		errBundle = multierror.Append(errBundle, err)

		err = tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.PatcherFailed("Error connecting to MariaDB"))
		errBundle = multierror.Append(errBundle, err)

		return ctrl.Result{}, fmt.Errorf("error creating MariaDB client: %v", errBundle)
	}
	defer mdbClient.Close()

	var errBundle *multierror.Error
	err = tr.WrappedReconciler.Reconcile(ctx, mdbClient)
	errBundle = multierror.Append(errBundle, err)

	err = tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.PatcherWithError(err))
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating %s: %v", resource.GetName(), err)
	}
	return ctrl.Result{}, nil
}

func waitForMariaDB(ctx context.Context, client client.Client, resource Resource,
	mariadb *mariadbv1alpha1.MariaDB) error {
	if !resource.MariaDBRef().WaitForIt {
		return nil
	}
	var mariadbErr *multierror.Error
	healthy, err := health.IsMariaDBHealthy(ctx, client, mariadb, health.EndpointPolicyAll)
	if err != nil {
		mariadbErr = multierror.Append(mariadbErr, err)
	}
	if !healthy {
		mariadbErr = multierror.Append(mariadbErr, errors.New("MariaDB not healthy"))
	}
	return mariadbErr.ErrorOrNil()
}
