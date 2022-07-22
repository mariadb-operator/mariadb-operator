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
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	userFinalizerName = "user.database.mmontes.io/finalizer"
)

func (r *UserMariaDBReconciler) addFinalizer(ctx context.Context, user *databasev1alpha1.UserMariaDB) error {
	if controllerutil.ContainsFinalizer(user, userFinalizerName) {
		return nil
	}
	patch := ctrlClient.MergeFrom(user.DeepCopy())
	controllerutil.AddFinalizer(user, userFinalizerName)

	if err := r.Client.Patch(ctx, user, patch); err != nil {
		return fmt.Errorf("error adding finalizer to UserMariaDB: %v", err)
	}
	return nil
}

func (r *UserMariaDBReconciler) finalize(ctx context.Context, user *databasev1alpha1.UserMariaDB,
	mdbClient *mariadbclient.Client) error {
	if !controllerutil.ContainsFinalizer(user, userFinalizerName) {
		return nil
	}

	if err := mdbClient.DropUser(ctx, user.Name); err != nil {
		return fmt.Errorf("error dropping user in MariaDB: %v", err)
	}

	patch := ctrlClient.MergeFrom(user.DeepCopy())
	controllerutil.RemoveFinalizer(user, userFinalizerName)

	if err := r.Client.Patch(ctx, user, patch); err != nil {
		return fmt.Errorf("error removing finalizer to UserMariaDB: %v", err)
	}
	return nil
}
