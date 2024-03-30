package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSecretBuilder(t *testing.T) {
	builder := newTestBuilder()
	tests := []struct {
		name     string
		opts     SecretOpts
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "no meta",
			opts: SecretOpts{
				Metadata: &mariadbv1alpha1.Metadata{},
				Key: types.NamespacedName{
					Name: "configmap",
				},
				Data: map[string][]byte{
					"password": []byte("test"),
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "no meta",
			opts: SecretOpts{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
				Key: types.NamespacedName{
					Name: "configmap",
				},
				Data: map[string][]byte{
					"password": []byte("test"),
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
			configMap, err := builder.BuildSecret(tt.opts, &mariadbv1alpha1.MariaDB{})
			if err != nil {
				t.Fatalf("unexpected error building Secret: %v", err)
			}
			assertMeta(t, &configMap.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
