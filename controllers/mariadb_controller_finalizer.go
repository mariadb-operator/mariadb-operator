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
	"errors"
	"fmt"
	"time"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	mariadbFinalizerName = "mariadb.database.mmontes.io/finalizer"
	finalizerInterval    = 1 * time.Second
	finalizerTimeout     = 30 * time.Second
)

func (r *MariaDBReconciler) addFinalizer(ctx context.Context, mariaDb *databasev1alpha1.MariaDB) error {
	if controllerutil.ContainsFinalizer(mariaDb, mariadbFinalizerName) {
		return nil
	}
	patch := ctrlClient.MergeFrom(mariaDb.DeepCopy())
	controllerutil.AddFinalizer(mariaDb, mariadbFinalizerName)

	if err := r.Client.Patch(ctx, mariaDb, patch); err != nil {
		return fmt.Errorf("error adding finalizer to MariaDB: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) finalize(ctx context.Context, mariaDb *databasev1alpha1.MariaDB) error {
	if !controllerutil.ContainsFinalizer(mariaDb, mariadbFinalizerName) {
		return nil
	}
	if mariaDb.Spec.Metrics == nil {
		if err := r.removeMariaDbFinalizer(ctx, mariaDb); err != nil {
			return fmt.Errorf("error removing finalizer from MariaDB: %v", err)
		}
		return nil
	}

	if err := r.finalizeMetricCredentials(ctx, mariaDb); err != nil {
		return fmt.Errorf("error finalizing metric credentials: %v", err)
	}

	if err := r.removeMariaDbFinalizer(ctx, mariaDb); err != nil {
		return fmt.Errorf("error removing finalizer from MariaDB: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) removeMariaDbFinalizer(ctx context.Context, mariaDb *databasev1alpha1.MariaDB) error {
	patch := ctrlClient.MergeFrom(mariaDb.DeepCopy())
	controllerutil.RemoveFinalizer(mariaDb, mariadbFinalizerName)

	if err := r.Client.Patch(ctx, mariaDb, patch); err != nil {
		return fmt.Errorf("error removing finalizer from MariaDB: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) finalizeMetricCredentials(ctx context.Context,
	mariadb *databasev1alpha1.MariaDB) error {
	if mariadb.Spec.Metrics == nil || !mariadb.IsReady() {
		return nil
	}
	if !mariadb.IsReady() {
		return errors.New("error finalizing metrics: MariaDB is not ready")
	}

	if err := r.deleteUser(ctx, mariadb); err != nil {
		return fmt.Errorf("error finalizing MariaDB: %v", err)
	}
	if err := r.deleteGrant(ctx, mariadb); err != nil {
		return fmt.Errorf("error finalizing GrantMariaDB: %v", err)
	}

	return nil
}

func (r *MariaDBReconciler) deleteUser(ctx context.Context, mariadb *databasev1alpha1.MariaDB) error {
	key := exporterKey(mariadb)
	var user databasev1alpha1.UserMariaDB
	if err := r.Get(ctx, key, &user); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error getting UserMariaDB: %v", err)
	}

	if err := r.Delete(ctx, &user); err != nil {
		return fmt.Errorf("error deleting UserMariaDB: %v", err)
	}

	err := wait.PollImmediateWithContext(ctx, finalizerInterval, finalizerTimeout, func(ctx context.Context) (bool, error) {
		if err := r.Get(ctx, key, &user); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("error waiting for UserMariaDB to be deleted: %v", err)
	}

	return nil
}

func (r *MariaDBReconciler) deleteGrant(ctx context.Context, mariadb *databasev1alpha1.MariaDB) error {
	key := exporterKey(mariadb)
	var grant databasev1alpha1.GrantMariaDB
	if err := r.Get(ctx, key, &grant); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error getting GrantMariaDB: %v", err)
	}

	if err := r.Delete(ctx, &grant); err != nil {
		return fmt.Errorf("error deleting GrantMariaDB: %v", err)
	}

	err := wait.PollImmediateWithContext(ctx, finalizerInterval, finalizerTimeout, func(ctx context.Context) (bool, error) {
		if err := r.Get(ctx, key, &grant); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("error waiting for GrantMariaDB to be deleted: %v", err)
	}

	return nil
}
