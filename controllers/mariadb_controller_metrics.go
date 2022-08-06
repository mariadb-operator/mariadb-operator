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
	"github.com/mmontes11/mariadb-operator/pkg/builders"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	exporterPrivileges = []string{
		"SELECT",
		"PROCESS",
		// TODO: check MariaDB version and use 'REPLICATION CLIENT' instead
		// see: https://mariadb.com/kb/en/grant/#binlog-monitor
		"BINLOG MONITOR",
		"SLAVE MONITOR",
	}
	passwordSecretKey = "password"
	dsnSecretKey      = "dsn"
)

func (r *MariaDBReconciler) createMetricsCredentials(ctx context.Context,
	mariaDb *databasev1alpha1.MariaDB) (*corev1.SecretKeySelector, error) {
	user, err := r.createMetricsUser(ctx, mariaDb)
	if err != nil {
		return nil, fmt.Errorf("error creating metrics UserMariaDB: %v", err)
	}

	if err := r.createMetricsGrant(ctx, mariaDb, user); err != nil {
		return nil, fmt.Errorf("error creating metrics GrantMariaDB: %v", err)
	}

	dsn, err := r.createMetricsDsn(ctx, mariaDb, user)
	if err != nil {
		return nil, fmt.Errorf("error creating metrics DSN: %v", err)
	}
	return dsn, nil
}

func (r *MariaDBReconciler) createMetricsUser(ctx context.Context,
	mariadb *databasev1alpha1.MariaDB) (*databasev1alpha1.UserMariaDB, error) {
	key := metricsKey(mariadb)
	var existingUser databasev1alpha1.UserMariaDB
	if err := r.Get(ctx, key, &existingUser); err == nil {
		return &existingUser, nil
	}

	secretKeySelector, err := r.createMetricsPasswordSecret(ctx, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error creating user password Secret: %v", err)
	}

	opts := builders.UserOpts{
		Key:                  key,
		PasswordSecretKeyRef: *secretKeySelector,
		MaxUserConnections:   3,
	}
	user := builders.BuildUserMariaDB(mariadb, opts)
	if err := controllerutil.SetControllerReference(mariadb, user, r.Scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to UserMariaDB: %v", err)
	}

	if err := r.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("error creating UserMariaDB: %v", err)
	}
	return user, nil
}

func (r *MariaDBReconciler) createMetricsGrant(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	user *databasev1alpha1.UserMariaDB) error {
	key := metricsKey(mariadb)
	var grantMariaDB databasev1alpha1.GrantMariaDB
	if err := r.Get(ctx, key, &grantMariaDB); err == nil {
		return nil
	}

	opts := builders.GrantOpts{
		Key:         key,
		Privileges:  exporterPrivileges,
		Database:    "*",
		Table:       "*",
		Username:    user.Name,
		GrantOption: false,
	}
	grant := builders.BuildGrantMariaDB(mariadb, opts)
	if err := controllerutil.SetControllerReference(mariadb, grant, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to GrantMariaDB: %v", err)
	}

	if err := r.Create(ctx, grant); err != nil {
		return fmt.Errorf("error creating GrantMariaDB: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) createMetricsPasswordSecret(ctx context.Context,
	mariadb *databasev1alpha1.MariaDB) (*corev1.SecretKeySelector, error) {
	password, err := password.Generate(16, 4, 0, false, false)
	if err != nil {
		return nil, fmt.Errorf("error generating password: %v", err)
	}

	opts := builders.SecretOpts{
		Key: passwordKey(mariadb),
		Data: map[string][]byte{
			passwordSecretKey: []byte(password),
		},
	}
	secret := builders.BuildSecret(mariadb, opts)
	if err := controllerutil.SetControllerReference(mariadb, secret, r.Scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to password Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("error creating password Secret: %v", err)
	}

	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
		Key: passwordSecretKey,
	}, nil
}

func (r *MariaDBReconciler) createMetricsDsn(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	user *databasev1alpha1.UserMariaDB) (*corev1.SecretKeySelector, error) {
	key := dsnKey(mariadb)
	var existingSecret v1.Secret
	if err := r.Get(ctx, key, &existingSecret); err == nil {
		return &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: existingSecret.Name,
			},
			Key: dsnSecretKey,
		}, nil
	}

	password, err := r.RefResolver.ReadSecretKeyRef(ctx, user.Spec.PasswordSecretKeyRef, mariadb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting password: %v", err)
	}
	mdbOpts := mariadbclient.Opts{
		Username: user.Name,
		Password: password,
		Host:     mariadb.Name,
		Port:     mariadb.Spec.Port,
	}
	dsn, err := mariadbclient.BuildDSN(mdbOpts)
	if err != nil {
		return nil, fmt.Errorf("error building DSN: %v", err)
	}

	secretOpts := builders.SecretOpts{
		Key: key,
		Data: map[string][]byte{
			dsnSecretKey: []byte(dsn),
		},
	}
	secret := builders.BuildSecret(mariadb, secretOpts)
	if err := controllerutil.SetControllerReference(mariadb, secret, r.Scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to DSN Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("error creating DSN Secret: %v", err)
	}
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
		Key: dsnSecretKey,
	}, nil
}

func metricsKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-metrics", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func passwordKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-password", metricsKey(mariadb).Name),
		Namespace: mariadb.Namespace,
	}
}

func dsnKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-dsn", metricsKey(mariadb).Name),
		Namespace: mariadb.Namespace,
	}
}
