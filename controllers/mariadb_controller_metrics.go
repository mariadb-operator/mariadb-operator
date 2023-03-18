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

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	exporterPrivileges = []string{
		"SELECT",
		"PROCESS",
		"REPLICATION CLIENT",
		"REPLICA MONITOR",
		"SLAVE MONITOR",
	}
	passwordSecretKey = "password"
	dsnSecretKey      = "dsn"
)

func (r *MariaDBReconciler) reconcileMetricsCredentials(ctx context.Context,
	mariaDb *mariadbv1alpha1.MariaDB) (*corev1.SecretKeySelector, error) {
	user, err := r.createMetricsUser(ctx, mariaDb)
	if err != nil {
		return nil, fmt.Errorf("error creating metrics User: %v", err)
	}

	if err := r.createMetricsGrant(ctx, mariaDb, user); err != nil {
		return nil, fmt.Errorf("error creating metrics Grant: %v", err)
	}

	dsn, err := r.createMetricsDsn(ctx, mariaDb, user)
	if err != nil {
		return nil, fmt.Errorf("error creating metrics DSN: %v", err)
	}
	return dsn, nil
}

func (r *MariaDBReconciler) createMetricsUser(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) (*mariadbv1alpha1.User, error) {
	key := metricsKey(mariadb)
	var existingUser mariadbv1alpha1.User
	if err := r.Get(ctx, key, &existingUser); err == nil {
		return &existingUser, nil
	}

	secretKeySelector, err := r.createMetricsPasswordSecret(ctx, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error creating user password Secret: %v", err)
	}

	opts := builder.UserOpts{
		Key:                  key,
		PasswordSecretKeyRef: *secretKeySelector,
		MaxUserConnections:   3,
	}
	user, err := r.Builder.BuildUser(mariadb, opts)
	if err != nil {
		return nil, fmt.Errorf("error building User: %v", err)
	}

	if err := r.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("error creating User: %v", err)
	}
	return user, nil
}

func (r *MariaDBReconciler) createMetricsGrant(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	user *mariadbv1alpha1.User) error {
	key := metricsKey(mariadb)
	var existingGrant mariadbv1alpha1.Grant
	if err := r.Get(ctx, key, &existingGrant); err == nil {
		return nil
	}

	opts := builder.GrantOpts{
		Key:         key,
		Privileges:  exporterPrivileges,
		Database:    "*",
		Table:       "*",
		Username:    user.Name,
		GrantOption: false,
	}
	grant, err := r.Builder.BuildGrant(mariadb, opts)
	if err != nil {
		return fmt.Errorf("error building Grant: %v", err)
	}

	if err := r.Create(ctx, grant); err != nil {
		return fmt.Errorf("error creating Grant: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) createMetricsPasswordSecret(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) (*corev1.SecretKeySelector, error) {
	password, err := password.Generate(16, 4, 0, false, false)
	if err != nil {
		return nil, fmt.Errorf("error generating password: %v", err)
	}

	opts := builder.SecretOpts{
		Key: passwordKey(mariadb),
		Data: map[string][]byte{
			passwordSecretKey: []byte(password),
		},
		Labels: labels.NewLabelsBuilder().WithMariaDB(mariadb).Build(),
	}
	secret, err := r.Builder.BuildSecret(opts, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error building password Secret: %v", err)
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

func (r *MariaDBReconciler) createMetricsDsn(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	user *mariadbv1alpha1.User) (*corev1.SecretKeySelector, error) {
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

	password, err := r.RefResolver.SecretKeyRef(ctx, user.Spec.PasswordSecretKeyRef, mariadb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting password: %v", err)
	}
	mdbOpts := mariadbclient.Opts{
		Username: user.Name,
		Password: password,
		Host:     mariadbclient.FQDN(mariadb),
		Port:     mariadb.Spec.Port,
	}
	dsn, err := mariadbclient.BuildDSN(mdbOpts)
	if err != nil {
		return nil, fmt.Errorf("error building DSN: %v", err)
	}

	secretOpts := builder.SecretOpts{
		Key: key,
		Data: map[string][]byte{
			dsnSecretKey: []byte(dsn),
		},
		Labels: labels.NewLabelsBuilder().WithMariaDB(mariadb).Build(),
	}
	secret, err := r.Builder.BuildSecret(secretOpts, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error building DNS Secret: %v", err)
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

func metricsKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-metrics", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func passwordKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-password", metricsKey(mariadb).Name),
		Namespace: mariadb.Namespace,
	}
}

func dsnKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-dsn", metricsKey(mariadb).Name),
		Namespace: mariadb.Namespace,
	}
}
