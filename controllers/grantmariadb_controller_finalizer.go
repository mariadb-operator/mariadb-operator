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
	grantFinalizerName = "grant.database.mmontes.io/finalizer"
)

func (r *GrantMariaDBReconciler) addFinalizer(ctx context.Context, grant *databasev1alpha1.GrantMariaDB) error {
	if controllerutil.ContainsFinalizer(grant, grantFinalizerName) {
		return nil
	}
	patch := ctrlClient.MergeFrom(grant.DeepCopy())
	controllerutil.AddFinalizer(grant, grantFinalizerName)

	if err := r.Client.Patch(ctx, grant, patch); err != nil {
		return fmt.Errorf("error adding finalizer to GrantMariaDB: %v", err)
	}
	return nil
}

func (r *GrantMariaDBReconciler) finalize(ctx context.Context, grant *databasev1alpha1.GrantMariaDB,
	mdbClient *mariadbclient.Client) error {
	if !controllerutil.ContainsFinalizer(grant, grantFinalizerName) {
		return nil
	}

	if err := r.revoke(ctx, grant, mdbClient); err != nil {
		return fmt.Errorf("error revoking grant: %v", err)
	}

	patch := ctrlClient.MergeFrom(grant.DeepCopy())
	controllerutil.RemoveFinalizer(grant, grantFinalizerName)

	if err := r.Client.Patch(ctx, grant, patch); err != nil {
		return fmt.Errorf("error removing finalizer to GrantMariaDB: %v", err)
	}
	return nil
}
