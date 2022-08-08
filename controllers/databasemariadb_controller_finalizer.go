/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/controllers/template"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	databaseFinalizerName = "database.database.mmontes.io/finalizer"
)

type wrappedDatabaseFinalizer struct {
	client.Client
	database *databasev1alpha1.DatabaseMariaDB
}

func newWrappedDatabaseFinalizer(client client.Client, database *databasev1alpha1.DatabaseMariaDB) template.WrappedFinalizer {
	return &wrappedDatabaseFinalizer{
		Client:   client,
		database: database,
	}
}

func (wf *wrappedDatabaseFinalizer) AddFinalizer(ctx context.Context) error {
	if !wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.database, func(database *databasev1alpha1.DatabaseMariaDB) {
		controllerutil.AddFinalizer(database, databaseFinalizerName)
	})
}

func (wf *wrappedDatabaseFinalizer) RemoveFinalizer(ctx context.Context) error {
	if wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.database, func(database *databasev1alpha1.DatabaseMariaDB) {
		controllerutil.RemoveFinalizer(database, databaseFinalizerName)
	})
}

func (wr *wrappedDatabaseFinalizer) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(wr.database, databaseFinalizerName)
}

func (wf *wrappedDatabaseFinalizer) Reconcile(ctx context.Context, mdbClient *mariadbclient.Client) error {
	if err := mdbClient.DropDatabase(ctx, wf.database.Name); err != nil {
		return fmt.Errorf("error dropping database in MariaDB: %v", err)
	}
	return nil
}

func (wr *wrappedDatabaseFinalizer) patch(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB,
	patchFn func(*databasev1alpha1.DatabaseMariaDB)) error {
	patch := ctrlClient.MergeFrom(database.DeepCopy())
	patchFn(database)

	if err := wr.Client.Patch(ctx, database, patch); err != nil {
		return fmt.Errorf("error patching DatabaseMariaDB finalizer: %v", err)
	}
	return nil

}
