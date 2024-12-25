package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileTLS(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsTLSEnabled() {
		return ctrl.Result{}, nil
	}
	if err := r.reconcileTLSCerts(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileTLSCABundle(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileTLSConfig(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileTLSCABundle(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
	logger := log.FromContext(ctx).WithName("ca-bundle")

	caBundleKeySelector := mdb.TLSCABundleSecretKeyRef()
	serverCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mdb.TLSServerCASecretKey().Name,
		},
		Key: pki.CACertKey,
	}
	clientCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mdb.TLSClientCASecretKey().Name,
		},
		Key: pki.CACertKey,
	}
	caKeySelectors := []mariadbv1alpha1.SecretKeySelector{
		caBundleKeySelector,
		serverCAKeySelector,
		clientCAKeySelector,
	}
	var caBundles [][]byte

	for _, caKeySelector := range caKeySelectors {
		ca, err := r.RefResolver.SecretKeyRef(ctx, caKeySelector, mdb.Namespace)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error getting CA Secret \"%s\": %v", caKeySelector.Name, err)
			}
			logger.V(1).Info("CA Secret not found", "secret-name", caKeySelector.Name)
		}
		caBundles = append(caBundles, []byte(ca))
	}

	bundle, err := pki.BundleCertificatePEMs(
		caBundles,
		pki.WithLogger(logger),
		pki.WithSkipExpired(true),
	)
	if err != nil {
		return fmt.Errorf("error creating CA bundle: %v", err)
	}

	secretReq := secret.SecretRequest{
		Metadata: []*mariadbv1alpha1.Metadata{mdb.Spec.InheritMetadata},
		Owner:    mdb,
		Key: types.NamespacedName{
			Name:      caBundleKeySelector.Name,
			Namespace: mdb.Namespace,
		},
		Data: map[string][]byte{
			caBundleKeySelector.Key: bundle,
		},
	}
	return r.SecretReconciler.Reconcile(ctx, &secretReq)
}

func (r *MariaDBReconciler) reconcileTLSCerts(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
	// tls := ptr.Deref(mdb.Spec.TLS, mariadbv1alpha1.TLS{})

	// caBundleKeySelector := mdb.TLSCABundleSecretKeyRef() // falback to ServerCASecretRef if bundle not yet created (1st time)
	// shouldIssueCA := tls.ServerCASecretRef == nil
	// shouldIssueCert := tls.ServerCertSecretRef == nil

	return nil
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
		SSLCert: builderpki.ServerCertPath,
		SSLKey:  builderpki.ServerKeyPath,
		SSLCA:   builderpki.CACertPath,
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

func (r *MariaDBReconciler) getTLSAnnotations(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (map[string]string, error) {
	if !mariadb.IsTLSEnabled() {
		return nil, nil
	}

	annotations, err := r.getTLSClientAnnotations(ctx, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error getting client annotations: %v", err)
	}

	serverCertKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mariadb.TLSServerCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	serverCert, err := r.RefResolver.SecretKeyRef(ctx, serverCertKeySelector, mariadb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting server cert: %v", err)
	}
	annotations[metadata.TLSServerCertAnnotation] = hash(serverCert)

	return annotations, nil
}

func (r *MariaDBReconciler) getTLSClientAnnotations(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (map[string]string, error) {
	if !mariadb.IsTLSEnabled() {
		return nil, nil
	}
	annotations := make(map[string]string)

	ca, err := r.RefResolver.SecretKeyRef(ctx, mariadb.TLSCABundleSecretKeyRef(), mariadb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting CA bundle: %v", err)
	}
	annotations[metadata.TLSCAAnnotation] = hash(ca)

	clientCertKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mariadb.TLSClientCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	clientCert, err := r.RefResolver.SecretKeyRef(ctx, clientCertKeySelector, mariadb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting client cert: %v", err)
	}
	annotations[metadata.TLSClientCertAnnotation] = hash(clientCert)

	return annotations, nil
}

func (r *MariaDBReconciler) getTLSStatus(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (*mariadbv1alpha1.MariaDBTLSStatus, error) {
	if !mdb.IsTLSEnabled() {
		return nil, nil
	}
	var tlsStatus mariadbv1alpha1.MariaDBTLSStatus

	certStatus, err := getCertificateStatus(ctx, r.RefResolver, mdb.TLSCABundleSecretKeyRef(), mdb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting CA bundle status: %v", err)
	}
	tlsStatus.CABundle = certStatus

	secretKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mdb.TLSServerCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	certStatus, err = getCertificateStatus(ctx, r.RefResolver, secretKeySelector, mdb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting Server certificate status: %v", err)
	}
	tlsStatus.ServerCert = ptr.To(certStatus[0])

	secretKeySelector = mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mdb.TLSClientCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	certStatus, err = getCertificateStatus(ctx, r.RefResolver, secretKeySelector, mdb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting Client certificate status: %v", err)
	}
	tlsStatus.ClientCert = ptr.To(certStatus[0])

	return &tlsStatus, nil
}

func getCertificateStatus(ctx context.Context, refResolver *refresolver.RefResolver, selector mariadbv1alpha1.SecretKeySelector,
	namespace string) ([]mariadbv1alpha1.CertificateStatus, error) {
	secret, err := refResolver.SecretKeyRef(ctx, selector, namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting Secret: %v", err)
	}

	certs, err := pki.ParseCertificates([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("error getting certificates: %v", err)
	}
	if len(certs) == 0 {
		return nil, errors.New("no certificates were found")
	}

	status := make([]mariadbv1alpha1.CertificateStatus, len(certs))
	for i, cert := range certs {
		status[i] = mariadbv1alpha1.CertificateStatus{
			NotAfter:  metav1.NewTime(cert.NotAfter),
			NotBefore: metav1.NewTime(cert.NotBefore),
			Subject:   cert.Subject.String(),
			Issuer:    cert.Issuer.String(),
		}
	}
	return status, nil
}
