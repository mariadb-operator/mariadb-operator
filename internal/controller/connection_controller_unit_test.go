package controller

import (
	"context"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/pki"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetSqlOptsSkipsMariadbTLSForNonTLSMaxScaleConnection(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &ConnectionReconciler{
		Client:      fakeClient,
		RefResolver: refresolver.New(fakeClient),
	}

	conn := &mariadbv1alpha1.Connection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "conn-maxscale",
			Namespace: "test",
		},
	}
	refs := &mariadbv1alpha1.ConnectionRefs{
		MaxScale: &mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "maxscale",
				Namespace: "test",
			},
		},
		MariaDB: &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mariadb",
				Namespace: "test",
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				TLS: &mariadbv1alpha1.TLS{
					Enabled: true,
				},
			},
		},
	}

	opts, err := reconciler.getSqlOpts(context.Background(), conn, refs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.TLSCACert) != 0 {
		t.Fatalf("expected MaxScale connection without TLS to omit CA bundle")
	}
	if opts.MariadbName != "" {
		t.Fatalf("expected MaxScale connection without TLS to avoid MariaDB TLS config")
	}
	if opts.MaxscaleName != "" {
		t.Fatalf("expected MaxScale TLS name to be unset when MaxScale TLS is disabled")
	}
}

func TestGetSqlOptsUsesExplicitClientCertForTLSMaxScaleConnection(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	maxScale := &mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "maxscale",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MaxScaleSpec{
			TLS: &mariadbv1alpha1.MaxScaleTLS{
				Enabled: true,
			},
		},
	}
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maxScale.TLSCABundleSecretKeyRef().Name,
			Namespace: maxScale.Namespace,
		},
		Data: map[string][]byte{
			pki.CACertKey: []byte("maxscale-ca"),
		},
	}
	clientSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client-cert",
			Namespace: "test",
		},
		Data: map[string][]byte{
			pki.TLSCertKey: []byte("client-cert-pem"),
			pki.TLSKeyKey:  []byte("client-key-pem"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(maxScale, caSecret, clientSecret).
		Build()
	reconciler := &ConnectionReconciler{
		Client:      fakeClient,
		RefResolver: refresolver.New(fakeClient),
	}

	conn := &mariadbv1alpha1.Connection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "conn-maxscale-tls",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.ConnectionSpec{
			TLSClientCertSecretRef: &mariadbv1alpha1.LocalObjectReference{
				Name: clientSecret.Name,
			},
		},
	}
	refs := &mariadbv1alpha1.ConnectionRefs{
		MaxScale: maxScale,
		MariaDB: &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mariadb",
				Namespace: "test",
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				TLS: &mariadbv1alpha1.TLS{
					Enabled: true,
				},
			},
		},
	}

	opts, err := reconciler.getSqlOpts(context.Background(), conn, refs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(opts.TLSCACert) != "maxscale-ca" {
		t.Fatalf("unexpected MaxScale CA bundle: %q", string(opts.TLSCACert))
	}
	if opts.MaxscaleName != maxScale.Name {
		t.Fatalf("unexpected MaxScale name: %q", opts.MaxscaleName)
	}
	if opts.ClientName != clientSecret.Name {
		t.Fatalf("unexpected client certificate name: %q", opts.ClientName)
	}
	if string(opts.TLSClientCert) != "client-cert-pem" {
		t.Fatalf("unexpected client certificate contents: %q", string(opts.TLSClientCert))
	}
	if string(opts.TLSClientPrivateKey) != "client-key-pem" {
		t.Fatalf("unexpected client private key contents: %q", string(opts.TLSClientPrivateKey))
	}
}

func TestGetSqlOptsFallsBackToMariadbClientCertForTLSMaxScaleConnection(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	maxScale := &mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "maxscale",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MaxScaleSpec{
			TLS: &mariadbv1alpha1.MaxScaleTLS{
				Enabled: true,
			},
		},
	}
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			TLS: &mariadbv1alpha1.TLS{
				Enabled: true,
			},
		},
	}
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maxScale.TLSCABundleSecretKeyRef().Name,
			Namespace: maxScale.Namespace,
		},
		Data: map[string][]byte{
			pki.CACertKey: []byte("maxscale-ca"),
		},
	}
	mariadbClientSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.TLSClientCertSecretKey().Name,
			Namespace: mariadb.Namespace,
		},
		Data: map[string][]byte{
			pki.TLSCertKey: []byte("mariadb-client-cert-pem"),
			pki.TLSKeyKey:  []byte("mariadb-client-key-pem"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(maxScale, caSecret, mariadbClientSecret).
		Build()
	reconciler := &ConnectionReconciler{
		Client:      fakeClient,
		RefResolver: refresolver.New(fakeClient),
	}

	conn := &mariadbv1alpha1.Connection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "conn-maxscale-tls",
			Namespace: "test",
		},
	}
	refs := &mariadbv1alpha1.ConnectionRefs{
		MaxScale: maxScale,
		MariaDB:  mariadb,
	}

	opts, err := reconciler.getSqlOpts(context.Background(), conn, refs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(opts.TLSCACert) != "maxscale-ca" {
		t.Fatalf("unexpected MaxScale CA bundle: %q", string(opts.TLSCACert))
	}
	if opts.ClientName != mariadbClientSecret.Name {
		t.Fatalf("unexpected client certificate name: %q", opts.ClientName)
	}
	if string(opts.TLSClientCert) != "mariadb-client-cert-pem" {
		t.Fatalf("unexpected client certificate contents: %q", string(opts.TLSClientCert))
	}
	if string(opts.TLSClientPrivateKey) != "mariadb-client-key-pem" {
		t.Fatalf("unexpected client private key contents: %q", string(opts.TLSClientPrivateKey))
	}
}
