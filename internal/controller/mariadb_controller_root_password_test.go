package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	mariadbsql "github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
)

func TestUpdateRootUserPassword(t *testing.T) {
	type alterCall struct {
		Account string
		Opts    mariadbsql.CreateUserOpts
	}

	t.Run("updates both root accounts with unlimited user connections", func(t *testing.T) {
		var calls []alterCall
		err := updateRootUserPassword(
			context.Background(),
			func(_ context.Context, account string, createUserOpts ...mariadbsql.CreateUserOpt) error {
				var opts mariadbsql.CreateUserOpts
				for _, setOpt := range createUserOpts {
					setOpt(&opts)
				}
				calls = append(calls, alterCall{Account: account, Opts: opts})
				return nil
			},
			"new-secret",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []alterCall{
			{
				Account: "'root'@'localhost'",
				Opts: mariadbsql.CreateUserOpts{
					IdentifiedBy:       "new-secret",
					MaxUserConnections: 0,
				},
			},
			{
				Account: "'root'@'%'",
				Opts: mariadbsql.CreateUserOpts{
					IdentifiedBy:       "new-secret",
					MaxUserConnections: 0,
				},
			},
		}
		if diff := cmp.Diff(want, calls); diff != "" {
			t.Errorf("unexpected alter user calls (-want +got):\n%s", diff)
		}
	})

	t.Run("aggregates errors across root accounts", func(t *testing.T) {
		wantErr := errors.New("alter failed")
		err := updateRootUserPassword(
			context.Background(),
			func(_ context.Context, account string, _ ...mariadbsql.CreateUserOpt) error {
				if account == "'root'@'%'" {
					return wantErr
				}
				return nil
			},
			"new-secret",
		)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}
