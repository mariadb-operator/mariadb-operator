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
	"k8s.io/apimachinery/pkg/types"
)

func (r *MariaDBReconciler) bootstrapFromBackup(ctx context.Context, mariadb *databasev1alpha1.MariaDB) error {
	if mariadb.Spec.BootstrapFromBackup == nil || !mariadb.IsReady() || mariadb.IsBootstrapped() {
		return nil
	}
	key := bootstrapRestoreKey(mariadb)
	var existingRestore databasev1alpha1.RestoreMariaDB
	if err := r.Get(ctx, key, &existingRestore); err == nil {
		return nil
	}

	restore, err := r.Builder.BuildRestoreMariaDb(
		mariadb,
		mariadb.Spec.BootstrapFromBackup.BackupRef,
		key,
	)
	if err != nil {
		return fmt.Errorf("error building RestoreMariaDB: %v", err)
	}

	if err := r.Create(ctx, restore); err != nil {
		return fmt.Errorf("error creating bootstrapping restore Job: %v", err)
	}
	return nil
}

func bootstrapRestoreKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("bootstrap-restore-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
