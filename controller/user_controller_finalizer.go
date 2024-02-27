package controller

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/sql"
	sqlClient "github.com/mariadb-operator/mariadb-operator/pkg/sql"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	userFinalizerName = "user.k8s.mariadb.com/finalizer"
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

func (wf *wrappedUserFinalizer) Reconcile(ctx context.Context, mdbClient *sqlClient.Client) error {
	if err := mdbClient.DropUser(ctx, wf.user.AccountName()); err != nil {
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
