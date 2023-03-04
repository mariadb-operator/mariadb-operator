package sql

import (
	"context"
	"fmt"

	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type SqlFinalizer struct {
	RefResolver *refresolver.RefResolver

	WrappedFinalizer WrappedFinalizer
}

func NewSqlFinalizer(rr *refresolver.RefResolver, wf WrappedFinalizer) Finalizer {
	return &SqlFinalizer{
		RefResolver:      rr,
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

func (tf *SqlFinalizer) Finalize(ctx context.Context, resource Resource) error {
	if !tf.WrappedFinalizer.ContainsFinalizer() {
		return nil
	}

	mariaDb, err := tf.RefResolver.MariaDB(ctx, resource.MariaDBRef(), resource.GetNamespace())
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := tf.WrappedFinalizer.RemoveFinalizer(ctx); err != nil {
				return fmt.Errorf("error removing %s finalizer: %v", resource.GetName(), err)
			}
			return nil
		}
		return fmt.Errorf("error getting MariaDB: %v", err)
	}

	// TODO: connection pooling. See https://github.com/mmontes11/mariadb-operator/issues/7.
	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariaDb, tf.RefResolver)
	if err != nil {
		return fmt.Errorf("error connecting to MariaDB: %v", err)
	}
	defer mdbClient.Close()

	if err := tf.WrappedFinalizer.Reconcile(ctx, mdbClient); err != nil {
		return fmt.Errorf("error reconciling in TemplateFinalizer: %v", err)
	}

	if err := tf.WrappedFinalizer.RemoveFinalizer(ctx); err != nil {
		return fmt.Errorf("error removing finalizer in TemplateFinalizer: %v", err)
	}
	return nil
}
