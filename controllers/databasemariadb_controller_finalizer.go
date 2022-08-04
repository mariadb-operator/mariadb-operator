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
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	databaseFinalizerName = "database.database.mmontes.io/finalizer"
)

func (r *DatabaseMariaDBReconciler) addFinalizer(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB) error {
	if !controllerutil.ContainsFinalizer(database, databaseFinalizerName) {
		return nil
	}
	return r.patch(ctx, database, func(database *databasev1alpha1.DatabaseMariaDB) {
		controllerutil.AddFinalizer(database, databaseFinalizerName)
	})
}

func (r *DatabaseMariaDBReconciler) removeFinalizer(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB) error {
	if controllerutil.ContainsFinalizer(database, databaseFinalizerName) {
		return nil
	}
	return r.patch(ctx, database, func(database *databasev1alpha1.DatabaseMariaDB) {
		controllerutil.RemoveFinalizer(database, databaseFinalizerName)
	})
}

func (r *DatabaseMariaDBReconciler) finalize(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB) error {
	if !controllerutil.ContainsFinalizer(database, databaseFinalizerName) {
		return nil
	}

	mariaDb, err := r.RefResolver.GetMariaDB(ctx, database.Spec.MariaDBRef, database.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.removeFinalizer(ctx, database); err != nil {
				return fmt.Errorf("error removing DatabaseMariaDB finalizer: %v", err)
			}
			return nil
		}
		return fmt.Errorf("error getting MariaDB: %v", err)
	}

	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariaDb, r.RefResolver)
	if err != nil {
		return fmt.Errorf("error connecting to MariaDB: %v", err)
	}

	if err := mdbClient.DropDatabase(ctx, database.Name); err != nil {
		return fmt.Errorf("error dropping database in MariaDB: %v", err)
	}

	if err := r.removeFinalizer(ctx, database); err != nil {
		return fmt.Errorf("error removing DatabaseMariaDB finalizer: %v", err)
	}
	return nil
}

func (r *DatabaseMariaDBReconciler) patch(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB,
	patchFn func(*databasev1alpha1.DatabaseMariaDB)) error {
	patch := ctrlClient.MergeFrom(database.DeepCopy())
	patchFn(database)

	if err := r.Client.Patch(ctx, database, patch); err != nil {
		return fmt.Errorf("error patching DatabaseMariaDB finalizer: %v", err)
	}
	return nil

}
