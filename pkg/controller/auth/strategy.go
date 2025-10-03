package auth

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Strategy interface {
	reconcileUser(ctx context.Context, userOpts builder.UserOpts) (ctrl.Result, error)
	// reconcileGrant will reconcile a new grant. userOpts are passed here to do associations only and should not be used to create
	// resources.
	reconcileGrant(ctx context.Context, userOpts builder.UserOpts, grantOpts builder.GrantOpts) (ctrl.Result, error)
}
