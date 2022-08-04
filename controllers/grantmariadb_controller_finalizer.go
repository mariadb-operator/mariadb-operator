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
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	grantFinalizerName = "grant.database.mmontes.io/finalizer"
)

func (r *GrantMariaDBReconciler) addFinalizer(ctx context.Context, grant *databasev1alpha1.GrantMariaDB) error {
	if !controllerutil.ContainsFinalizer(grant, grantFinalizerName) {
		return nil
	}
	return r.patch(ctx, grant, func(gmd *databasev1alpha1.GrantMariaDB) {
		controllerutil.AddFinalizer(grant, grantFinalizerName)
	})
}

func (r *GrantMariaDBReconciler) removeFinalizer(ctx context.Context, grant *databasev1alpha1.GrantMariaDB) error {
	if controllerutil.ContainsFinalizer(grant, grantFinalizerName) {
		return nil
	}
	return r.patch(ctx, grant, func(gmd *databasev1alpha1.GrantMariaDB) {
		controllerutil.RemoveFinalizer(grant, grantFinalizerName)
	})
}

func (r *GrantMariaDBReconciler) finalize(ctx context.Context, grant *databasev1alpha1.GrantMariaDB) error {
	if !controllerutil.ContainsFinalizer(grant, grantFinalizerName) {
		return nil
	}

	mariaDb, err := r.RefResolver.GetMariaDB(ctx, grant.Spec.MariaDBRef, grant.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.removeFinalizer(ctx, grant); err != nil {
				return fmt.Errorf("error removing GrantMariaDB finalizer: %v", err)
			}
			return nil
		}
		return fmt.Errorf("error getting MariaDB: %v", err)
	}

	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariaDb, r.RefResolver)
	if err != nil {
		return fmt.Errorf("error connecting to MariaDB: %v", err)
	}

	if err := r.revoke(ctx, grant, mdbClient); err != nil {
		return fmt.Errorf("error revoking grant: %v", err)
	}

	patch := ctrlClient.MergeFrom(grant.DeepCopy())
	controllerutil.RemoveFinalizer(grant, grantFinalizerName)

	if err := r.Client.Patch(ctx, grant, patch); err != nil {
		return fmt.Errorf("error removing GrantMariaDB finalizer: %v", err)
	}
	return nil
}

func (r *GrantMariaDBReconciler) patch(ctx context.Context, grant *databasev1alpha1.GrantMariaDB,
	patchFn func(*databasev1alpha1.GrantMariaDB)) error {
	patch := client.MergeFrom(grant.DeepCopy())
	patchFn(grant)

	if err := r.Client.Patch(ctx, grant, patch); err != nil {
		return fmt.Errorf("error patching GrantMariaDB: %v", err)
	}
	return nil
}

func (r *GrantMariaDBReconciler) revoke(ctx context.Context, grant *databasev1alpha1.GrantMariaDB,
	mdbClient *mariadbclient.Client) error {
	err := wait.PollImmediateWithContext(ctx, 1*time.Second, 5*time.Second, func(ctx context.Context) (bool, error) {
		var user databasev1alpha1.UserMariaDB
		if err := r.Get(ctx, userKey(grant), &user); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
	// User does not exist
	if err == nil {
		return nil
	}
	if err != nil && !errors.Is(err, wait.ErrWaitTimeout) {
		return fmt.Errorf("error checking if user exists in MariaDB: %v", err)
	}

	opts := mariadbclient.GrantOpts{
		Privileges:  grant.Spec.Privileges,
		Database:    grant.Spec.Database,
		Table:       grant.Spec.Table,
		Username:    grant.Spec.Username,
		GrantOption: grant.Spec.GrantOption,
	}
	if err := mdbClient.Revoke(ctx, opts); err != nil {
		return fmt.Errorf("error revoking grant in MariaDB: %v", err)
	}
	return nil
}
