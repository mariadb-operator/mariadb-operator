package auth

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Strategy interface {
	reconcileUser(ctx context.Context, userOpts builder.UserOpts) (ctrl.Result, error)
	reconcileGrant(ctx context.Context, grantOpts builder.GrantOpts) (ctrl.Result, error)
	reconcileUserPassword(ctx context.Context) (ctrl.Result, error)
}
