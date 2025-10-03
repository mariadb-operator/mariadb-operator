package auth

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	ctrl "sigs.k8s.io/controller-runtime"
)

// @NOTE: This is intentionally left as an empty struct, due to where the package is located.
// This would also allow for future changes if needed
type AuthReconciler struct{}

// ReconcileUserGrant will reconcile a User and a Grant Curstom Resource. This involves multiple requeues
func (r *AuthReconciler) ReconcileUserGrant(ctx context.Context, userOpts builder.UserOpts,
	grantOpts []builder.GrantOpts, strategy Strategy) (ctrl.Result, error) {
	if strategy == nil {
		return ctrl.Result{}, fmt.Errorf("no strategy passed to AuthController")
	}

	if result, err := strategy.reconcileUser(ctx, userOpts); !result.IsZero() || err != nil {
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling User: %v", err)
		}
		return result, err
	}

	for _, gops := range grantOpts {
		if len(gops.Privileges) > 0 {
			if result, err := strategy.reconcileGrant(ctx, userOpts, gops); !result.IsZero() || err != nil {
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("error reconciling Grant: %v", err)
				}

				return result, err
			}
		}
	}

	return ctrl.Result{}, nil
}
