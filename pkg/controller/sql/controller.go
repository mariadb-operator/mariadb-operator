package sql

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	ctrl "sigs.k8s.io/controller-runtime"
)

type SqlReconciler struct {
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready

	WrappedReconciler WrappedReconciler
	Finalizer         Finalizer
}

func NewSqlReconciler(rr *refresolver.RefResolver, cr *conditions.Ready, wr WrappedReconciler, f Finalizer) Reconciler {
	return &SqlReconciler{
		RefResolver:       rr,
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

	var mariaDbErr *multierror.Error
	mariaDb, err := tr.RefResolver.MariaDB(ctx, resource.MariaDBRef(), resource.GetNamespace())
	if err != nil {
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.PatcherRefResolver(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	if resource.MariaDBRef().WaitForIt && !mariaDb.IsReady() {
		if err := tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.PatcherFailed("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching %s: %v", resource.GetName(), err)
		}
		return ctrl.Result{}, errors.New("MariaDB not ready")
	}

	// TODO: connection pooling. See https://github.com/mariadb-operator/mariadb-operator/issues/7.
	mdbClient, err := mariadbclient.NewRootClient(ctx, mariaDb, tr.RefResolver)
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
