package sql

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/pkg/sql"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SqlFinalizer struct {
	Client      client.Client
	RefResolver *refresolver.RefResolver

	WrappedFinalizer WrappedFinalizer
}

func NewSqlFinalizer(client client.Client, wf WrappedFinalizer) Finalizer {
	return &SqlFinalizer{
		Client:           client,
		RefResolver:      refresolver.New(client),
		WrappedFinalizer: wf,
	}
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

	if result, err := waitForMariaDB(ctx, tf.Client, resource, mariadb); !result.IsZero() || err != nil {
		return result, err
	}

	// TODO: connection pooling. See https://github.com/mariadb-operator/mariadb-operator/issues/7.
	mdbClient, err := sqlClient.NewClientWithMariaDB(ctx, mariadb, tf.RefResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error connecting to MariaDB: %v", err)
	}
	defer mdbClient.Close()

	if err := tf.WrappedFinalizer.Reconcile(ctx, mdbClient); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling in TemplateFinalizer: %v", err)
	}

	if err := tf.WrappedFinalizer.RemoveFinalizer(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error removing finalizer in TemplateFinalizer: %v", err)
	}
	return ctrl.Result{}, nil
}
