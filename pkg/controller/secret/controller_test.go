package secret

import (
	"context"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

type writeCounter struct {
	creates int
	updates int
	patches int
}

func newTestSecretReconciler(t *testing.T, objs ...client.Object) (*SecretReconciler, *writeCounter) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, mariadbv1alpha1.AddToScheme(scheme))

	counter := &writeCounter{}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				counter.creates++
				return c.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				counter.updates++
				return c.Update(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				counter.patches++
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()

	r, err := NewSecretReconciler(c, builder.NewBuilder(scheme, nil, nil))
	require.NoError(t, err)
	return r, counter
}

func testSecretOwner() *mariadbv1alpha1.MariaDB {
	return &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "default",
			UID:       "mariadb-uid",
		},
	}
}

func testSecretRequest(name string, data map[string][]byte) *SecretRequest {
	return &SecretRequest{
		Owner: testSecretOwner(),
		Metadata: []*mariadbv1alpha1.Metadata{
			{
				Labels: map[string]string{
					"k8s.mariadb.com/test": "test",
				},
			},
		},
		Key: types.NamespacedName{
			Name:      name,
			Namespace: "default",
		},
		Data: data,
	}
}

func TestSecretReconcilerSkipsUnchangedSecrets(t *testing.T) {
	tests := []struct {
		name string
		req  *SecretRequest
	}{
		{
			name: "CA bundle",
			req: testSecretRequest("mariadb-ca-bundle", map[string][]byte{
				"ca.crt": []byte("-----BEGIN CERTIFICATE-----\nfoo\n-----END CERTIFICATE-----\n"),
			}),
		},
		{
			name: "metrics config",
			req: testSecretRequest("mariadb-metrics-config", map[string][]byte{
				"exporter.cnf": []byte("[client]\nuser=mariadb-metrics\npassword=MariaDB11!\n"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, counter := newTestSecretReconciler(t)

			require.NoError(t, r.Reconcile(context.Background(), tt.req))
			assert.Equal(t, writeCounter{creates: 1}, *counter, "initial reconcile must create the Secret once")

			for range 3 {
				require.NoError(t, r.Reconcile(context.Background(), tt.req))
			}
			assert.Equal(t, writeCounter{creates: 1}, *counter, "unchanged reconciles must not patch or update the Secret")

			var secret corev1.Secret
			require.NoError(t, r.Get(context.Background(), tt.req.Key, &secret))
			assert.Equal(t, tt.req.Data, secret.Data)
		})
	}
}

func TestSecretReconcilerReconcile(t *testing.T) {
	key := types.NamespacedName{
		Name:      "mariadb-ca-bundle",
		Namespace: "default",
	}
	existingSecret := func(data map[string][]byte) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
				Labels: map[string]string{
					"k8s.mariadb.com/test": "test",
					"example.com/external": "true",
				},
				Annotations: map[string]string{
					"example.com/external": "true",
				},
			},
			Data: data,
		}
	}

	tests := []struct {
		name       string
		existing   *corev1.Secret
		req        *SecretRequest
		wantWrites writeCounter
		wantData   map[string][]byte
	}{
		{
			name:       "unchanged data",
			existing:   existingSecret(map[string][]byte{"ca.crt": []byte("foo")}),
			req:        testSecretRequest(key.Name, map[string][]byte{"ca.crt": []byte("foo")}),
			wantWrites: writeCounter{},
			wantData:   map[string][]byte{"ca.crt": []byte("foo")},
		},
		{
			name:       "nil and empty data",
			existing:   existingSecret(nil),
			req:        testSecretRequest(key.Name, map[string][]byte{}),
			wantWrites: writeCounter{},
			wantData:   nil,
		},
		{
			name:       "empty value",
			existing:   existingSecret(map[string][]byte{"ca.crt": {}}),
			req:        testSecretRequest(key.Name, map[string][]byte{"ca.crt": {}}),
			wantWrites: writeCounter{},
			wantData:   map[string][]byte{"ca.crt": {}},
		},
		{
			name:       "changed value",
			existing:   existingSecret(map[string][]byte{"ca.crt": []byte("foo")}),
			req:        testSecretRequest(key.Name, map[string][]byte{"ca.crt": []byte("bar")}),
			wantWrites: writeCounter{patches: 1},
			wantData:   map[string][]byte{"ca.crt": []byte("bar")},
		},
		{
			name:       "added key",
			existing:   existingSecret(map[string][]byte{"ca.crt": []byte("foo")}),
			req:        testSecretRequest(key.Name, map[string][]byte{"ca.crt": []byte("foo"), "tls.crt": []byte("bar")}),
			wantWrites: writeCounter{patches: 1},
			wantData:   map[string][]byte{"ca.crt": []byte("foo"), "tls.crt": []byte("bar")},
		},
		{
			name:       "removed key",
			existing:   existingSecret(map[string][]byte{"ca.crt": []byte("foo"), "tls.crt": []byte("bar")}),
			req:        testSecretRequest(key.Name, map[string][]byte{"ca.crt": []byte("foo")}),
			wantWrites: writeCounter{patches: 1},
			wantData:   map[string][]byte{"ca.crt": []byte("foo")},
		},
		{
			name:     "skip if exists",
			existing: existingSecret(map[string][]byte{"ca.crt": []byte("foo")}),
			req: func() *SecretRequest {
				req := testSecretRequest(key.Name, map[string][]byte{"ca.crt": []byte("bar")})
				req.SkipIfExists = true
				return req
			}(),
			wantWrites: writeCounter{},
			wantData:   map[string][]byte{"ca.crt": []byte("foo")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, counter := newTestSecretReconciler(t, tt.existing)

			require.NoError(t, r.Reconcile(context.Background(), tt.req))
			assert.Equal(t, tt.wantWrites, *counter)

			var secret corev1.Secret
			require.NoError(t, r.Get(context.Background(), key, &secret))
			assert.Equal(t, tt.wantData, secret.Data)
			assert.Equal(t, tt.existing.Labels, secret.Labels, "labels must be preserved")
			assert.Equal(t, tt.existing.Annotations, secret.Annotations, "annotations must be preserved")
		})
	}
}
