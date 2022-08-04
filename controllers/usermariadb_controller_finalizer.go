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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	userFinalizerName = "user.database.mmontes.io/finalizer"
)

func (r *UserMariaDBReconciler) addFinalizer(ctx context.Context, user *databasev1alpha1.UserMariaDB) error {
	if controllerutil.ContainsFinalizer(user, userFinalizerName) {
		return nil
	}
	return r.patch(ctx, user, func(user *databasev1alpha1.UserMariaDB) {
		controllerutil.AddFinalizer(user, userFinalizerName)
	})
}

func (r *UserMariaDBReconciler) removeFinalizer(ctx context.Context, user *databasev1alpha1.UserMariaDB) error {
	if !controllerutil.ContainsFinalizer(user, userFinalizerName) {
		return nil
	}
	return r.patch(ctx, user, func(user *databasev1alpha1.UserMariaDB) {
		controllerutil.RemoveFinalizer(user, userFinalizerName)
	})
}

func (r *UserMariaDBReconciler) finalize(ctx context.Context, user *databasev1alpha1.UserMariaDB) error {
	if !controllerutil.ContainsFinalizer(user, userFinalizerName) {
		return nil
	}

	mariaDb, err := r.RefResolver.GetMariaDB(ctx, user.Spec.MariaDBRef, user.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.removeFinalizer(ctx, user); err != nil {
				return fmt.Errorf("error removing UserMariaDB finalizer: %v", err)
			}
			return nil
		}
		return fmt.Errorf("error getting MariaDB: %v", err)
	}

	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariaDb, r.RefResolver)
	if err != nil {
		return fmt.Errorf("error connecting to MariaDB: %v", err)
	}

	if err := mdbClient.DropUser(ctx, user.Name); err != nil {
		return fmt.Errorf("error dropping user in MariaDB: %v", err)
	}

	if err := r.removeFinalizer(ctx, user); err != nil {
		return fmt.Errorf("error removing UserMariaDB finalizer: %v", err)
	}
	return nil
}

func (r *UserMariaDBReconciler) patch(ctx context.Context, user *databasev1alpha1.UserMariaDB,
	patchFn func(*databasev1alpha1.UserMariaDB)) error {
	patch := client.MergeFrom(user.DeepCopy())
	patchFn(user)

	if err := r.Client.Patch(ctx, user, patch); err != nil {
		return fmt.Errorf("error removing finalizer to UserMariaDB: %v", err)
	}
	return nil
}
