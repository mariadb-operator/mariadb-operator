package command

import (
	"errors"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestConnectionFlags(t *testing.T) {
	tests := []struct {
		name     string
		opts     *CommandOpts
		mariadb  *mariadbv1alpha1.MariaDB
		flagOpts []ConnectionFlagOpt
		want     string
		wantErr  error
	}{
		{
			name:    "missing UserEnv",
			opts:    &CommandOpts{PasswordEnv: "PASS"},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantErr: errors.New("UserEnv must be set"),
		},
		{
			name: "missing PasswordEnv",
			opts: &CommandOpts{UserEnv: "USER"},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantErr: errors.New("PasswordEnv must be set"),
		},
		{
			name: "basic flags with standalone",
			opts: &CommandOpts{UserEnv: "USER", PasswordEnv: "PASS"},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
				},
			},
			want: "--user=${USER} --password=${PASS} --host=test.default.svc.cluster.local --port=3306",
		},
		{
			name: "basic flags with Galera",
			opts: &CommandOpts{UserEnv: "USER", PasswordEnv: "PASS"},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			want: "--user=${USER} --password=${PASS} --host=test-primary.default.svc.cluster.local --port=3306",
		},
		{
			name: "with database",
			opts: &CommandOpts{UserEnv: "USER", PasswordEnv: "PASS", Database: ptr.To("test")},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			want: "--user=${USER} --password=${PASS} --host=test-primary.default.svc.cluster.local --port=3306 --database=test",
		},
		{
			name: "with host override",
			opts: &CommandOpts{UserEnv: "USER", PasswordEnv: "PASS"},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			flagOpts: []ConnectionFlagOpt{WithHostConnectionFlag("custom-host")},
			want:     "--user=${USER} --password=${PASS} --host=custom-host --port=3306",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConnectionFlags(tt.opts, tt.mariadb, tt.flagOpts...)
			if tt.wantErr != nil {
				if err == nil || err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
