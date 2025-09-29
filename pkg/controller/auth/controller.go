package auth

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	ctrl "sigs.k8s.io/controller-runtime"
)

type AuthReconciler struct{}

// ReconcileUserGrant will reconcile a user and a grant. This involves multiple requeues
// Defaults:
// Waits for user creation
// Doesn't wait for Grant creation
// Does not generate password
func (r *AuthReconciler) ReconcileUserGrant(ctx context.Context, userOpts builder.UserOpts, grantOpts []builder.GrantOpts, strategy Strategy) (ctrl.Result, error) {
	if strategy == nil {
		return ctrl.Result{}, fmt.Errorf("no strategy passed to AuthController")
	}

	if result, err := strategy.reconcileUser(ctx, userOpts); !result.IsZero() || err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling User: %v", err)
	}

	for _, gops := range grantOpts {
		if len(gops.Privileges) > 0 {
			if result, err := strategy.reconcileGrant(ctx, gops); !result.IsZero() || err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling Grant: %v", err)
			}
		}
	}

	return ctrl.Result{}, nil
}
