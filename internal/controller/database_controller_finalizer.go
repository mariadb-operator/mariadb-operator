package controller

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/sql"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	databaseFinalizerName = "database.k8s.mariadb.com/finalizer"
)

type wrappedDatabaseFinalizer struct {
	client.Client
	database *mariadbv1alpha1.Database
}

func newWrappedDatabaseFinalizer(client client.Client, database *mariadbv1alpha1.Database) sql.WrappedFinalizer {
	return &wrappedDatabaseFinalizer{
		Client:   client,
		database: database,
	}
}

func (wf *wrappedDatabaseFinalizer) AddFinalizer(ctx context.Context) error {
	if wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.database, func(database *mariadbv1alpha1.Database) {
		controllerutil.AddFinalizer(database, databaseFinalizerName)
	})
}

func (wf *wrappedDatabaseFinalizer) RemoveFinalizer(ctx context.Context) error {
	if !wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.database, func(database *mariadbv1alpha1.Database) {
		controllerutil.RemoveFinalizer(database, databaseFinalizerName)
	})
}

func (wf *wrappedDatabaseFinalizer) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(wf.database, databaseFinalizerName)
}

func (wf *wrappedDatabaseFinalizer) Reconcile(ctx context.Context, mdbClient *sqlClient.Client) error {
	if err := mdbClient.DropDatabase(ctx, wf.database.DatabaseNameOrDefault()); err != nil {
		return fmt.Errorf("error dropping database in MariaDB: %v", err)
	}
	return nil
}

func (wf *wrappedDatabaseFinalizer) patch(ctx context.Context, database *mariadbv1alpha1.Database,
	patchFn func(*mariadbv1alpha1.Database)) error {
	patch := client.MergeFrom(database.DeepCopy())
	patchFn(database)

	if err := wf.Patch(ctx, database, patch); err != nil {
		return fmt.Errorf("error patching Database finalizer: %v", err)
	}
	return nil

}
