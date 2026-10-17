package sql

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/interfaces"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	readOnlyTargetMessage      = "MariaDB primary Service reaches a read-only node"
	readOnlyTargetRequeueDelay = 5 * time.Second
)

type readOnlyClient interface {
	IsSystemVariableEnabled(ctx context.Context, variable string) (bool, error)
}

// targetIsReadOnly reports whether the connection reached a read-only node of a replicated MariaDB. SQL resources
// reach it through the primary Service, whose selector follows status.currentPrimary and lags a primary change made
// by MaxScale. Until it catches up the Service still points at the demoted primary, where the superuser bypasses
// read_only: every statement then becomes a transaction only that node holds, and the demoted primary can no longer
// rejoin the new one from its own position.
func targetIsReadOnly(ctx context.Context, mariadb interfaces.MariaDBObject, client readOnlyClient) (bool, error) {
	mdb, ok := mariadb.(*mariadbv1alpha1.MariaDB)
	if !ok || !mdb.IsReplicationEnabled() {
		return false, nil
	}
	return client.IsSystemVariableEnabled(ctx, "read_only")
}

type SqlOptions struct {
	RequeueInterval  time.Duration
	RequeueMaxOffset time.Duration
	LogSql           bool
}

type SqlOpt func(*SqlOptions)

func WithRequeueInterval(interval time.Duration) SqlOpt {
	return func(opts *SqlOptions) {
		opts.RequeueInterval = interval
	}
}

func WithRequeueMaxOffset(offset time.Duration) SqlOpt {
	return func(opts *SqlOptions) {
		opts.RequeueMaxOffset = offset
	}
}

func WithLogSql(logSql bool) SqlOpt {
	return func(opts *SqlOptions) {
		opts.LogSql = logSql
	}
}

type SqlReconciler struct {
	Client         ctrlclient.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *condition.Ready

	WrappedReconciler WrappedReconciler
	Finalizer         Finalizer

	SqlOptions
}

func NewSqlReconciler(client ctrlclient.Client, cr *condition.Ready, wr WrappedReconciler, f Finalizer,
	opts ...SqlOpt) Reconciler {
	reconciler := &SqlReconciler{
		Client:            client,
		RefResolver:       refresolver.New(client),
		ConditionReady:    cr,
		WrappedReconciler: wr,
		Finalizer:         f,
		SqlOptions: SqlOptions{
			RequeueInterval:  10 * time.Hour,
			RequeueMaxOffset: 1 * time.Hour,
			LogSql:           false,
		},
	}
	for _, setOpt := range opts {
		setOpt(&reconciler.SqlOptions)
	}
	return reconciler
}

func (r *SqlReconciler) Reconcile(ctx context.Context, resource Resource) (ctrl.Result, error) {
	if resource.IsBeingDeleted() {
		if result, err := r.Finalizer.Finalize(ctx, resource); !result.IsZero() || err != nil {
			return result, err
		}
		return ctrl.Result{}, nil
	}

	mariadb, err := r.RefResolver.MariaDBObject(ctx, resource.MariaDBRef(), resource.GetNamespace())
	if err != nil {
		var errBundle *multierror.Error
		errBundle = multierror.Append(errBundle, err)

		err = r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherRefResolver(err, mariadb))
		errBundle = multierror.Append(errBundle, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", errBundle)
	}

	if result, err := waitForMariaDB(ctx, r.Client, mariadb, r.LogSql); !result.IsZero() || err != nil {
		var errBundle *multierror.Error

		if err != nil {
			errBundle = multierror.Append(errBundle, err)

			err := r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherWithError(err))
			errBundle = multierror.Append(errBundle, err)
		}

		return result, errBundle.ErrorOrNil()
	}

	if mdb, ok := mariadb.(*mariadbv1alpha1.MariaDB); ok {
		if mdb.HasPendingBinlogReplay() {
			if err := r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherFailed("MariaDB has pending binlog replay")); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
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

	readOnly, err := targetIsReadOnly(ctx, mariadb, mdbClient)
	if err != nil {
		return r.retryResult(ctx, resource, fmt.Errorf("error checking read_only: %v", err))
	}
	if readOnly {
		log.FromContext(ctx).Info("MariaDB primary Service reaches a read-only node. Requeuing...", "resource", resource.GetName())
		if err := r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherFailed(readOnlyTargetMessage)); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
		}
		return ctrl.Result{RequeueAfter: readOnlyTargetRequeueDelay}, nil
	}

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

	return r.requeueResult(ctx, resource, errBundle.ErrorOrNil())
}

func (r *SqlReconciler) retryResult(ctx context.Context, resource Resource, err error) (ctrl.Result, error) {
	if resource.RetryInterval() != nil {
		log.FromContext(ctx).Error(err, "Error reconciling SQL resource", "resource", resource.GetName())
		return ctrl.Result{RequeueAfter: resource.RetryInterval().Duration}, nil
	}
	if err != nil {
		if r.LogSql {
			log.FromContext(ctx).V(1).Info("Error reconciling SQL resource", "err", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *SqlReconciler) requeueResult(ctx context.Context, resource Resource, err error) (ctrl.Result, error) {
	if err != nil {
		log.FromContext(ctx).V(1).Info("Error reconciling SQL resource", "err", err)
		return ctrl.Result{Requeue: true}, nil
	}
	if resource.RequeueInterval() != nil {
		requeueInterval := r.addRequeueIntervalOffset(resource.RequeueInterval().Duration)
		if r.LogSql {
			log.FromContext(ctx).V(1).Info("Requeuing SQL resource")
		}
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}
	if r.RequeueInterval > 0 {
		requeueInterval := r.addRequeueIntervalOffset(r.RequeueInterval)
		if r.LogSql {
			log.FromContext(ctx).V(1).Info("Requeuing SQL resource")
		}
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}
	return ctrl.Result{}, nil
}

func (r *SqlReconciler) addRequeueIntervalOffset(duration time.Duration) time.Duration {
	if r.RequeueMaxOffset > 0 {
		return duration + time.Duration(rand.Int63()%int64(r.RequeueMaxOffset))
	}
	return duration
}

func waitForMariaDB(ctx context.Context, client ctrlclient.Client, mdb interfaces.MariaDBObject,
	logSql bool) (ctrl.Result, error) {
	switch mariadb := mdb.(type) {
	case *mariadbv1alpha1.ExternalMariaDB:
		return ctrl.Result{}, nil
	case *mariadbv1alpha1.MariaDB:
		return waitForMariaDBService(ctx, client, mariadb, logSql)
	}

	healthy, err := health.IsStatefulSetHealthy(
		ctx,
		client,
		ctrlclient.ObjectKeyFromObject(mdb),
		health.WithDesiredReplicas(mdb.GetReplicas()),
		health.WithPort(mdb.GetPort()),
		health.WithEndpointPolicy(health.EndpointPolicyAll),
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !healthy {
		if logSql {
			log.FromContext(ctx).V(1).Info("MariaDB unhealthy. Requeuing SQL resource")
		}
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func waitForMariaDBService(ctx context.Context, client ctrlclient.Client, mariadb *mariadbv1alpha1.MariaDB,
	logSql bool) (ctrl.Result, error) {
	serviceKey := ctrlclient.ObjectKeyFromObject(mariadb)
	if mariadb.IsHAEnabled() {
		serviceKey = mariadb.PrimaryServiceKey()
	}

	healthy, err := health.IsServiceHealthy(ctx, client, serviceKey)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !healthy {
		if logSql {
			log.FromContext(ctx).V(1).Info("MariaDB service not healthy. Requeuing SQL resource")
		}
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}
