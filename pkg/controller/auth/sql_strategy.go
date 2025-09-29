package auth

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SqlStrategy is an expansion strategy that builds upon the CrdStrategy.
// eg. `sqlStrategy, err := auth.NewSqlStrategy(client, crdStrategy)` is all you need to create the User and Grant resources, with SQL
type SqlStrategy struct {
	*CrdStrategy

	sqlClient *sql.Client
}

type SqlStrategyOpts func(strategy *SqlStrategy) error

// NewSqlStrategy creates a new SQL strategy.
// This strategy first creates the users after which the Custom Resources
// opts is left there for future expansion if needed
// @WARN: This should be used when we can't utilize the user controller, aka when the pod is not fully ready. In those cases, make sure to
// use the headless internal service.
func NewSqlStrategy(sqlClient *sql.Client, embeddedCrdStrategy *CrdStrategy, opts ...SqlStrategyOpts) (*SqlStrategy, error) {
	sqlStrategy := &SqlStrategy{CrdStrategy: embeddedCrdStrategy, sqlClient: sqlClient}

	for _, opt := range opts {
		if err := opt(sqlStrategy); err != nil {
			return nil, fmt.Errorf("error applying SqlStrategy options: %w", err)
		}
	}

	return sqlStrategy, nil
}

// reconcileUser will reconcile a User CR
// In this strategy however, we require the password to be either
// 1. Already created, in which case the SecretReconciler will return it
// 2. A secret ref to be passed with Generate = true, in which case it will be created.
// This is required because we create the user here.
// Also, the user creation happens before the CR is created to avoid any potential problems. This also means, no need to wait for user
// creation, it happens always
func (s *SqlStrategy) reconcileUser(ctx context.Context, userOpts builder.UserOpts) (ctrl.Result, error) {
	if s.userKey == (types.NamespacedName{}) || s.owner == nil {
		return ctrl.Result{}, fmt.Errorf("userKey or owner is not specified when reconciling user")
	}

	if !s.isReconcilePassword() {
		return ctrl.Result{}, fmt.Errorf("error creating user %v, no secretKeyRef or SecretReconciler passed", s.userKey)
	}

	password, err := s.reconcileUserPassword(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := s.createUser(ctx, userOpts.Host, userOpts.Name, password); err != nil {
		return ctrl.Result{}, err
	}

	if err := s.CrdStrategy.createUser(ctx, s.userKey, userOpts); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (s *SqlStrategy) createUser(ctx context.Context, host, username, password string) error {
	accountName := formatAccountName(username, host)
	exists, err := s.sqlClient.UserExists(ctx, username, host)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %w", err)
	}
	if exists {
		if err := s.sqlClient.AlterUser(ctx, accountName, sql.WithIdentifiedBy(password)); err != nil {
			return fmt.Errorf("error altering user: %w", err)
		}
	} else {
		if err := s.sqlClient.CreateUser(ctx, accountName, sql.WithIdentifiedBy(password)); err != nil {
			return fmt.Errorf("error creating user: %w", err)
		}
	}

	return nil
}

// reconcileGrant like reconcilerUser is not going to wait for the grant creation under any circumastances, as it is already created.
func (s *SqlStrategy) reconcileGrant(ctx context.Context, userOpts builder.UserOpts, grantOpts builder.GrantOpts) (ctrl.Result, error) {
	var grantKey types.NamespacedName
	if len(s.grantKeys) == 0 {
		return ctrl.Result{}, fmt.Errorf("error getting Grant key for grant. Not enough grantKeys given")
	}
	grantKey, s.grantKeys = s.grantKeys[0], s.grantKeys[1:]

	if grantKey == (types.NamespacedName{}) || s.owner == nil {
		return ctrl.Result{}, fmt.Errorf("grantKey or owner is not specified when reconciling user")
	}

	if err := s.createGrant(ctx, grantOpts.Host, userOpts.Name, grantOpts.Privileges, grantOpts.Database, grantOpts.Table); err != nil {
		return ctrl.Result{}, err
	}

	if err := s.CrdStrategy.createGrant(ctx, grantKey, grantOpts); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (s *SqlStrategy) createGrant(ctx context.Context, host, username string, privileges []string, database string, table string) error {
	accountName := formatAccountName(username, host)
	exists, err := s.sqlClient.UserExists(ctx, username, host)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("error trying to add grant to non-existent user")
	}
	if err := s.sqlClient.Grant(
		ctx,
		privileges,
		database,
		table,
		accountName,
	); err != nil {
		return fmt.Errorf("error adding grant: %v", err)
	}
	return nil
}

func formatAccountName(username, host string) string {
	return fmt.Sprintf("'%s'@'%s'", username, host)
}
