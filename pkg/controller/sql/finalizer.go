package sql

import (
	"context"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SqlFinalizer struct {
	Client      client.Client
	RefResolver *refresolver.RefResolver

	WrappedFinalizer WrappedFinalizer

	SqlOptions
}

func NewSqlFinalizer(client client.Client, wf WrappedFinalizer, opts ...SqlOpt) Finalizer {
	finalizer := &SqlFinalizer{
		Client:           client,
		RefResolver:      refresolver.New(client),
		WrappedFinalizer: wf,
		SqlOptions: SqlOptions{
			RequeueInterval: 30 * time.Second,
			LogSql:          false,
		},
	}
	for _, setOpt := range opts {
		setOpt(&finalizer.SqlOptions)
	}
	return finalizer
}

func (tf *SqlFinalizer) AddFinalizer(ctx context.Context) error {
	if tf.WrappedFinalizer.ContainsFinalizer() {
		return nil
	}
	if err := tf.WrappedFinalizer.AddFinalizer(ctx); err != nil {
		return fmt.Errorf("error adding finalizer in TemplateFinalizer: %v", err)
	}
	return nil
}

func (tf *SqlFinalizer) Finalize(ctx context.Context, resource Resource) (ctrl.Result, error) {
	if !tf.WrappedFinalizer.ContainsFinalizer() {
		return ctrl.Result{}, nil
	}

	mariadb, err := tf.RefResolver.MariaDB(ctx, resource.MariaDBRef(), resource.GetNamespace())
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := tf.WrappedFinalizer.RemoveFinalizer(ctx); err != nil {
				return ctrl.Result{}, fmt.Errorf("error removing %s finalizer: %v", resource.GetName(), err)
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", err)
	}

	// If MariaDB is being deleted, remove the finalizer
	if !mariadb.DeletionTimestamp.IsZero() {
		if err := tf.WrappedFinalizer.RemoveFinalizer(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("error removing %s finalizer: %v", resource.GetName(), err)
		}
		return ctrl.Result{}, nil
	}

	if result, err := waitForMariaDB(ctx, tf.Client, mariadb, tf.LogSql); !result.IsZero() || err != nil {
		return result, err
	}

	// TODO: connection pooling. See https://github.com/mariadb-operator/mariadb-operator/issues/7.
	mdbClient, err := sqlClient.NewClientWithMariaDB(ctx, mariadb, tf.RefResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error connecting to MariaDB: %v", err)
	}
	defer mdbClient.Close()

	cleanupPolicy := ptr.Deref(resource.CleanupPolicy(), mariadbv1alpha1.CleanupPolicyDelete)
	if cleanupPolicy == mariadbv1alpha1.CleanupPolicyDelete {
		log.FromContext(ctx).Info("Cleaning up SQL resource")

		if err := tf.WrappedFinalizer.Reconcile(ctx, mdbClient); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling in TemplateFinalizer: %v", err)
		}
	}

	if err := tf.WrappedFinalizer.RemoveFinalizer(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error removing finalizer in TemplateFinalizer: %v", err)
	}
	return ctrl.Result{}, nil
}
