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
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/sql"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	userFinalizerName = "user.mariadb.mmontes.io/finalizer"
)

type wrappedUserFinalizer struct {
	client.Client
	user *mariadbv1alpha1.User
}

func newWrappedUserFinalizer(client client.Client, user *mariadbv1alpha1.User) sql.WrappedFinalizer {
	return &wrappedUserFinalizer{
		Client: client,
		user:   user,
	}
}

func (wf *wrappedUserFinalizer) AddFinalizer(ctx context.Context) error {
	if wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.user, func(user *mariadbv1alpha1.User) {
		controllerutil.AddFinalizer(user, userFinalizerName)
	})
}

func (wf *wrappedUserFinalizer) RemoveFinalizer(ctx context.Context) error {
	if !wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.user, func(user *mariadbv1alpha1.User) {
		controllerutil.RemoveFinalizer(user, userFinalizerName)
	})
}

func (wf *wrappedUserFinalizer) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(wf.user, userFinalizerName)
}

func (wf *wrappedUserFinalizer) Reconcile(ctx context.Context, mdbClient *mariadbclient.Client) error {
	if err := mdbClient.DropUser(ctx, wf.user.Name); err != nil {
		return fmt.Errorf("error dropping user in MariaDB: %v", err)
	}
	return nil
}

func (wf *wrappedUserFinalizer) patch(ctx context.Context, user *mariadbv1alpha1.User,
	patchFn func(*mariadbv1alpha1.User)) error {
	patch := client.MergeFrom(user.DeepCopy())
	patchFn(user)

	if err := wf.Client.Patch(ctx, user, patch); err != nil {
		return fmt.Errorf("error removing finalizer to User: %v", err)
	}
	return nil
}
