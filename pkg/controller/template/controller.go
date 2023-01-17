package template

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	ctrl "sigs.k8s.io/controller-runtime"
)

type TemplateReconciler struct {
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready

	WrappedReconciler WrappedReconciler
	Finalizer         Finalizer
}

func NewTemplateReconciler(rr *refresolver.RefResolver, cr *conditions.Ready, wr WrappedReconciler, f Finalizer) Reconciler {
	return &TemplateReconciler{
		RefResolver:       rr,
		ConditionReady:    cr,
		WrappedReconciler: wr,
		Finalizer:         f,
	}
}

func (tr *TemplateReconciler) Reconcile(ctx context.Context, resource Resource) (ctrl.Result, error) {
	if resource.IsBeingDeleted() {
		if err := tr.Finalizer.Finalize(ctx, resource); err != nil {
			return ctrl.Result{}, fmt.Errorf("error finalizing %s: %v", resource.Meta().Name, err)
		}
		return ctrl.Result{}, nil
	}

	if err := tr.Finalizer.AddFinalizer(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error adding finalizer to %s: %v", resource.Meta().Name, err)
	}

	var mariaDbErr *multierror.Error
	mariaDb, err := tr.RefResolver.MariaDB(ctx, resource.MariaDBRef(), resource.Meta().Namespace)
	if err != nil {
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.RefResolverPatcher(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	if resource.MariaDBRef().WaitForIt && !mariaDb.IsReady() {
		if err := tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.FailedPatcher("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching %s: %v", resource.Meta().Name, err)
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	// TODO: connection pooling. See https://github.com/mmontes11/mariadb-operator/issues/7.
	var connErr *multierror.Error
	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariaDb, tr.RefResolver)
	if err != nil {
		connErr = multierror.Append(connErr, err)

		err = tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.FailedPatcher("Error connecting to MariaDB"))
		connErr = multierror.Append(connErr, err)

		return ctrl.Result{}, fmt.Errorf("error creating MariaDB client: %v", connErr)
	}
	defer mdbClient.Close()

	var errBundle *multierror.Error
	err = tr.WrappedReconciler.Reconcile(ctx, mdbClient)
	errBundle = multierror.Append(errBundle, err)

	err = tr.WrappedReconciler.PatchStatus(ctx, tr.ConditionReady.PatcherWithError(err))
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating %s: %v", resource.Meta().Name, err)
	}
	return ctrl.Result{}, nil
}
