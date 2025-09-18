package controller

import (
	"context"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/sql"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	grantFinalizerName = "grant.k8s.mariadb.com/finalizer"
)

type wrappedGrantFinalizer struct {
	client.Client
	grant *mariadbv1alpha1.Grant
}

func newWrappedGrantFinalizer(client client.Client, grant *mariadbv1alpha1.Grant) sql.WrappedFinalizer {
	return &wrappedGrantFinalizer{
		Client: client,
		grant:  grant,
	}
}

func (wf *wrappedGrantFinalizer) AddFinalizer(ctx context.Context) error {
	if wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.grant, func(gmd *mariadbv1alpha1.Grant) {
		controllerutil.AddFinalizer(wf.grant, grantFinalizerName)
	})
}

func (wf *wrappedGrantFinalizer) RemoveFinalizer(ctx context.Context) error {
	if !wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.grant, func(gmd *mariadbv1alpha1.Grant) {
		controllerutil.RemoveFinalizer(wf.grant, grantFinalizerName)
	})
}

func (wf *wrappedGrantFinalizer) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(wf.grant, grantFinalizerName)
}

func (wf *wrappedGrantFinalizer) Reconcile(ctx context.Context, mdbClient *sqlClient.Client) error {
	// When the User gets deleted first, there is nothing to be finalized in the Grant.
	// We can exit the finalizer reconciliation after attempting to get the User for 10s.
	// The rationale behind this is being able to delete invalid Grants pointing to an invalid user, without hanging in the finalizing logic.
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 10*time.Second, true, func(waitCtx context.Context) (bool, error) {
		exists, err := mdbClient.UserExists(ctx, wf.grant.Spec.Username, wf.grant.HostnameOrDefault())
		if err != nil {
			return true, fmt.Errorf("error checking if user exists in MariaDB: %v", err)
		}
		if !exists {
			return true, nil
		}
		return false, nil
	})
	// User does not exist after 10s, nothing to be finalized for this Grant.
	if err == nil {
		return nil
	}
	// An unexpected error occurred.
	if !wait.Interrupted(err) {
		return fmt.Errorf("error checking if user exists in MariaDB: %v", err)
	}

	var opts []sqlClient.GrantOption
	if wf.grant.Spec.GrantOption {
		opts = append(opts, sqlClient.WithGrantOption())
	}

	if err := mdbClient.Revoke(
		ctx,
		wf.grant.Spec.Privileges,
		wf.grant.Spec.Database,
		wf.grant.Spec.Table,
		wf.grant.AccountName(),
		opts...,
	); err != nil {
		return fmt.Errorf("error revoking grant in MariaDB: %v", err)
	}
	return nil
}

func (wf *wrappedGrantFinalizer) patch(ctx context.Context, grant *mariadbv1alpha1.Grant,
	patchFn func(*mariadbv1alpha1.Grant)) error {
	patch := client.MergeFrom(grant.DeepCopy())
	patchFn(grant)

	if err := wf.Patch(ctx, grant, patch); err != nil {
		return fmt.Errorf("error patching Grant: %v", err)
	}
	return nil
}
