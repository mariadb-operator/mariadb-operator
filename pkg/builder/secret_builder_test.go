package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestSecretBuilder(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tests := []struct {
		name     string
		opts     SecretOpts
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "no meta",
			opts: SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{},
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
			name: "single meta",
			opts: SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
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
		{
			name: "multiple meta",
			opts: SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
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
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
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
			assertObjectMeta(t, &configMap.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestBuildSecret(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tests := []struct {
		name    string
		opts    SecretOpts
		owner   metav1.Object
		wantErr bool
	}{
		{
			name: "no owner",
			opts: SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{},
				Key: types.NamespacedName{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			},
			owner:   nil,
			wantErr: false,
		},
		{
			name: "with owner",
			opts: SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{},
				Key: types.NamespacedName{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			},
			owner: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mariadb",
					Namespace: "default",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret, err := builder.BuildSecret(tt.opts, tt.owner)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.opts.Data, secret.Data)
			assert.Equal(t, tt.opts.Key.Name, secret.Name)
			assert.Equal(t, tt.opts.Key.Namespace, secret.Namespace)
			if tt.owner != nil {
				assert.True(t, controllerutil.HasControllerReference(secret))
			}
		})
	}
}
