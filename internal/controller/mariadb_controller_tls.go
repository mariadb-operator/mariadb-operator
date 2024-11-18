package controller

import (
	"bytes"
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileTLS(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsTLSEnabled() {
		return ctrl.Result{}, nil
	}
	if err := r.reconcileTLSCABundle(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileTLSConfig(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileTLSCABundle(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	serverCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mariadb.TLSServerCASecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	serverCA, err := r.RefResolver.SecretKeyRef(ctx, serverCAKeySelector, mariadb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting server: CA: %v", err)
	}

	clientCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mariadb.TLSClientCASecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	clientCA, err := r.RefResolver.SecretKeyRef(ctx, clientCAKeySelector, mariadb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting client CA: %v", err)
	}

	bundle, err := pki.BundleCertificatePEMs(log.FromContext(ctx), []byte(serverCA), []byte(clientCA))
	if err != nil {
		return fmt.Errorf("error creating CA bundle: %v", err)
	}

	secretKeyRef := mariadb.TLSCABundleSecretKeyRef()
	secretReq := secret.SecretRequest{
		Metadata: []*mariadbv1alpha1.Metadata{mariadb.Spec.InheritMetadata},
		Owner:    mariadb,
		Key: types.NamespacedName{
			Name:      secretKeyRef.Name,
			Namespace: mariadb.Namespace,
		},
		Data: map[string][]byte{
			secretKeyRef.Key: bundle,
		},
	}
	return r.SecretReconciler.Reconcile(ctx, &secretReq)
}

func (r *MariaDBReconciler) reconcileTLSConfig(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	configMapKeyRef := mariadb.TLSConfigMapKeyRef()

	tpl := createTpl("tls", `[mariadb]
ssl_cert = {{ .SSLCert }}
ssl_key = {{ .SSLKey }}
ssl_ca = {{ .SSLCA }}
require_secure_transport = true
`)
	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, struct {
		SSLCert string
		SSLKey  string
		SSLCA   string
	}{
		SSLCert: builderpki.MariadbTLSServerCertPath,
		SSLKey:  builderpki.MariadbTLSServerKeyPath,
		SSLCA:   builderpki.MariadbTLSCACertPath,
	})
	if err != nil {
		return fmt.Errorf("error rendering TLS config: %v", err)
	}

	configMapReq := configmap.ReconcileRequest{
		Metadata: mariadb.Spec.InheritMetadata,
		Owner:    mariadb,
		Key: types.NamespacedName{
			Name:      configMapKeyRef.Name,
			Namespace: mariadb.Namespace,
		},
		Data: map[string]string{
			configMapKeyRef.Key: buf.String(),
		},
	}
	return r.ConfigMapReconciler.Reconcile(ctx, &configMapReq)
}
