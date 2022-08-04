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
	"time"

	"github.com/mmontes11/mariadb-operator/api/v1alpha1"
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builders"
	"github.com/mmontes11/mariadb-operator/pkg/mariadb"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	exporterContainerName = "metrics"
	exporterPortName      = "metrics"
	exporterPort          = 9104
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

func (r *MariaDBReconciler) reconcileMetrics(ctx context.Context, req ctrl.Request,
	mariadb *databasev1alpha1.MariaDB) error {
	if mariadb.Spec.Metrics == nil || !mariadb.IsReady() {
		return nil
	}

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, req.NamespacedName, &sts); err != nil {
		return nil
	}
	var svc v1.Service
	if err := r.Get(ctx, req.NamespacedName, &svc); err != nil {
		return nil
	}
	if hasContainer(&sts, exporterContainerName) && hasPort(&svc, exporterPortName) {
		return nil
	}

	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariadb, r.RefResolver)
	if err != nil {
		return fmt.Errorf("error connecting to MariaDB: %v", err)
	}
	defer mdbClient.Close()

	dsn, err := r.createCredentials(ctx, mariadb, mdbClient)
	if err != nil {
		return fmt.Errorf("error creating credentials: %v", err)
	}

	patchSts := func(sts *appsv1.StatefulSet) {
		r.addExporterContainer(mariadb, sts, dsn)
	}
	if err := r.patchStatefulSet(ctx, &sts, patchSts); err != nil {
		return fmt.Errorf("error patching StatefulSet: %v", err)
	}

	if err := r.patchService(ctx, &svc, r.addExporterPort); err != nil {
		return fmt.Errorf("error patching Service: %v", err)
	}

	// TODO: create ServiceMonitor

	return nil
}

func (r *MariaDBReconciler) createCredentials(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	mdbClient *mariadb.Client) (*corev1.SecretKeySelector, error) {
	key := exporterKey(mariadb)
	if err := r.createUser(ctx, mariadb); err != nil {
		return nil, fmt.Errorf("error creating UserMariaDB: %v", err)
	}

	var user databasev1alpha1.UserMariaDB
	err := wait.PollImmediateWithContext(ctx, 1*time.Second, 10*time.Second, func(ctx context.Context) (bool, error) {
		if err := r.Get(ctx, key, &user); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return true, err
		}
		return user.IsReady(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for UserMariaDB to be ready: %v", err)
	}

	if err := r.createGrant(ctx, mariadb, &user); err != nil {
		return nil, fmt.Errorf("error creating GrantMariaDB: %v", err)
	}

	err = wait.PollImmediateWithContext(ctx, 1*time.Second, 10*time.Second, func(ctx context.Context) (bool, error) {
		var grant databasev1alpha1.GrantMariaDB
		if err := r.Get(ctx, key, &grant); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return true, err
		}
		return grant.IsReady(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for GrantMariaDB to be ready: %v", err)
	}

	dsn, err := r.createDsnSecret(ctx, mariadb, &user)
	if err != nil {
		return nil, fmt.Errorf("error creating DSN Secret: %v", err)
	}

	return dsn, nil
}

func (r *MariaDBReconciler) patchStatefulSet(ctx context.Context, sts *appsv1.StatefulSet, patchFn func(*appsv1.StatefulSet)) error {
	patch := client.MergeFrom(sts.DeepCopy())
	patchFn(sts)
	if err := r.Client.Patch(ctx, sts, patch); err != nil {
		return fmt.Errorf("error patching StatefulSet on API server: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) patchService(ctx context.Context, svc *v1.Service, patchFn func(*v1.Service)) error {
	patch := client.MergeFrom(svc.DeepCopy())
	patchFn(svc)
	if err := r.Client.Patch(ctx, svc, patch); err != nil {
		return fmt.Errorf("error patching Service on API server: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) addExporterContainer(mariadb *v1alpha1.MariaDB, sts *appsv1.StatefulSet, dsn *corev1.SecretKeySelector) {
	if hasContainer(sts, exporterContainerName) {
		return
	}
	opts := builders.ExporterOpts{
		ContainerName: exporterContainerName,
		PortName:      exporterPortName,
		Port:          exporterPort,
		DSN:           dsn,
	}
	container := builders.BuildExporterContainer(&mariadb.Spec.Metrics.Exporter, opts)

	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, container)
}

func (r *MariaDBReconciler) addExporterPort(svc *v1.Service) {
	if hasPort(svc, exporterPortName) {
		return
	}
	port := v1.ServicePort{
		Name: exporterPortName,
		Port: exporterPort,
	}

	svc.Spec.Ports = append(svc.Spec.Ports, port)
}

func (r *MariaDBReconciler) createUser(ctx context.Context, mariadb *databasev1alpha1.MariaDB) error {
	key := exporterKey(mariadb)
	var existingUser databasev1alpha1.UserMariaDB
	if err := r.Get(ctx, key, &existingUser); err == nil {
		return nil
	}

	secretKeySelector, err := r.createPasswordSecret(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error creating user password Secret: %v", err)
	}

	opts := builders.UserOpts{
		Key:                  key,
		PasswordSecretKeyRef: *secretKeySelector,
		MaxUserConnections:   3,
	}
	user := builders.BuildUserMariaDB(mariadb, opts)
	if err := controllerutil.SetControllerReference(mariadb, user, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to UserMariaDB: %v", err)
	}

	if err := r.Create(ctx, user); err != nil {
		return fmt.Errorf("error creating UserMariaDB: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) createGrant(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	user *databasev1alpha1.UserMariaDB) error {
	key := exporterKey(mariadb)
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

	return r.Create(ctx, grant)
}

func (r *MariaDBReconciler) createPasswordSecret(ctx context.Context,
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

func (r *MariaDBReconciler) createDsnSecret(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
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

func hasContainer(sts *appsv1.StatefulSet, containerName string) bool {
	for _, container := range sts.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return true
		}
	}
	return false
}

func hasPort(svc *v1.Service, portName string) bool {
	for _, port := range svc.Spec.Ports {
		if port.Name == portName {
			return true
		}
	}
	return false
}

func exporterKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func passwordKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter-password", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func dsnKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter-dsn", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
