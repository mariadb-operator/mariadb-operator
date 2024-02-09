package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/pkg/sql"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientpkg "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SqlReconciler struct {
	Client         client.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *condition.Ready

	WrappedReconciler WrappedReconciler
	Finalizer         Finalizer
	RequeueInterval   time.Duration
}

func NewSqlReconciler(client client.Client, cr *condition.Ready, wr WrappedReconciler, f Finalizer,
	requeueInterval time.Duration) Reconciler {
	return &SqlReconciler{
		Client:            client,
		RefResolver:       refresolver.New(client),
		ConditionReady:    cr,
		WrappedReconciler: wr,
		Finalizer:         f,
		RequeueInterval:   requeueInterval,
	}
}

func (r *SqlReconciler) Reconcile(ctx context.Context, resource Resource) (ctrl.Result, error) {
	if resource.IsBeingDeleted() {
		if result, err := r.Finalizer.Finalize(ctx, resource); !result.IsZero() || err != nil {
			return result, err
		}
		return ctrl.Result{}, nil
	}

	mariadb, err := r.RefResolver.MariaDB(ctx, resource.MariaDBRef(), resource.GetNamespace())
	if err != nil {
		var errBundle *multierror.Error
		errBundle = multierror.Append(errBundle, err)

		err = r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherRefResolver(err, mariadb))
		errBundle = multierror.Append(errBundle, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", errBundle)
	}

	if result, err := waitForMariaDB(ctx, r.Client, resource, mariadb); !result.IsZero() || err != nil {
		var errBundle *multierror.Error

		if err != nil {
			errBundle = multierror.Append(errBundle, err)

			err := r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherWithError(err))
			errBundle = multierror.Append(errBundle, err)
		}

		return result, errBundle.ErrorOrNil()
	}

	// TODO: connection pooling. See https://github.com/mariadb-operator/mariadb-operator/issues/7.
	mdbClient, err := sqlClient.NewClientWithMariaDB(ctx, mariadb, r.RefResolver)
	if err != nil {
		var errBundle *multierror.Error
		errBundle = multierror.Append(errBundle, err)

		msg := fmt.Sprintf("Error connecting to MariaDB: %v", err)
		err = r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherFailed(msg))
		errBundle = multierror.Append(errBundle, err)

		return r.retryResult(ctx, resource, errBundle)
	}
	defer mdbClient.Close()

	err = r.WrappedReconciler.Reconcile(ctx, mdbClient)
	var errBundle *multierror.Error
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		msg := fmt.Sprintf("Error creating %s: %v", resource.GetName(), err)
		err = r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherFailed(msg))
		errBundle = multierror.Append(errBundle, err)

		return r.retryResult(ctx, resource, errBundle)
	}

	if err = r.Finalizer.AddFinalizer(ctx); err != nil {
		errBundle = multierror.Append(errBundle, fmt.Errorf("error adding finalizer to %s: %v", resource.GetName(), err))
	}

	err = r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherWithError(errBundle.ErrorOrNil()))
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		return ctrl.Result{}, err
	}
	return r.requeueResult(ctx, resource)
}

func (r *SqlReconciler) retryResult(ctx context.Context, resource Resource, err error) (ctrl.Result, error) {
	if resource.RetryInterval() != nil {
		log.FromContext(ctx).Error(err, "Error reconciling SQL resource", "resource", resource.GetName())
		return ctrl.Result{RequeueAfter: resource.RetryInterval().Duration}, nil
	}
	return ctrl.Result{}, err
}

func (r *SqlReconciler) requeueResult(ctx context.Context, resource Resource) (ctrl.Result, error) {
	if resource.RequeueInterval() != nil {
		log.FromContext(ctx).V(1).Info("Requeuing SQL resource")
		return ctrl.Result{RequeueAfter: resource.RequeueInterval().Duration}, nil
	}
	if r.RequeueInterval > 0 {
		log.FromContext(ctx).V(1).Info("Requeuing SQL resource")
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}
	return ctrl.Result{}, nil
}

func waitForMariaDB(ctx context.Context, client client.Client, resource Resource,
	mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	healthy, err := health.IsStatefulSetHealthy(
		ctx,
		client,
		clientpkg.ObjectKeyFromObject(mdb),
		health.WithDesiredReplicas(mdb.Spec.Replicas),
		health.WithPort(mdb.Spec.Port),
		health.WithEndpointPolicy(health.EndpointPolicyAll),
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !healthy {
		log.FromContext(ctx).V(1).Info("MariaDB unhealthy. Requeuing SQL resource")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}
