package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestUserMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "user",
	}
	tests := []struct {
		name     string
		opts     UserOpts
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "no meta",
			opts: UserOpts{},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "meta",
			opts: UserOpts{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := builder.BuildUser(key, &mariadbv1alpha1.MariaDB{}, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error building User: %v", err)
			}
			assertObjectMeta(t, &user.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestUserCleanupPolicy(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "user",
	}
	tests := []struct {
		name              string
		opts              UserOpts
		wantCleanupPolicy *mariadbv1alpha1.CleanupPolicy
	}{
		{
			name:              "no cleanupPolicy",
			opts:              UserOpts{},
			wantCleanupPolicy: nil,
		},
		{
			name: "cleanupPolicy",
			opts: UserOpts{
				CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
			},
			wantCleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := builder.BuildUser(key, &mariadbv1alpha1.MariaDB{}, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error building User: %v", err)
			}
			if !reflect.DeepEqual(user.Spec.CleanupPolicy, tt.wantCleanupPolicy) {
				t.Errorf("unexpected cleanupPolicy: got: %v, want: %v", user.Spec.CleanupPolicy, tt.wantCleanupPolicy)
			}
		})
	}
}

func TestGrantMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "grant",
	}
	tests := []struct {
		name     string
		opts     GrantOpts
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "no meta",
			opts: GrantOpts{},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "meta",
			opts: GrantOpts{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grant, err := builder.BuildGrant(key, &mariadbv1alpha1.MariaDB{}, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error building Grant: %v", err)
			}
			assertObjectMeta(t, &grant.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestGrantCleanupPolicy(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "grant",
	}
	tests := []struct {
		name              string
		opts              GrantOpts
		wantCleanupPolicy *mariadbv1alpha1.CleanupPolicy
	}{
		{
			name:              "no cleanupPolicy",
			opts:              GrantOpts{},
			wantCleanupPolicy: nil,
		},
		{
			name: "cleanupPolicy",
			opts: GrantOpts{
				CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
			},
			wantCleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grant, err := builder.BuildGrant(key, &mariadbv1alpha1.MariaDB{}, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error building Grant: %v", err)
			}
			if !reflect.DeepEqual(grant.Spec.CleanupPolicy, tt.wantCleanupPolicy) {
				t.Errorf("unexpected cleanupPolicy: got: %v, want: %v", grant.Spec.CleanupPolicy, tt.wantCleanupPolicy)
			}
		})
	}
}
