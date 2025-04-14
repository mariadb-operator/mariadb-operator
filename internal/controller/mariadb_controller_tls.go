package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	builderpki "github.com/mariadb-operator/mariadb-operator/pkg/builder/pki"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	certctrl "github.com/mariadb-operator/mariadb-operator/pkg/controller/certificate"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/hash"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileTLS(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsTLSEnabled() {
		return ctrl.Result{}, nil
	}
	if result, err := r.reconcileTLSCerts(ctx, mariadb); !result.IsZero() || err != nil {
		return result, err
	}
	if err := r.reconcileTLSCABundle(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileTLSConfig(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileTLSCerts(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	tls := ptr.Deref(mdb.Spec.TLS, mariadbv1alpha1.TLS{})
	certHandler := newCertHandler(r.Client, r.RefResolver, mdb)

	serverCertOpts := []certctrl.CertReconcilerOpt{
		certctrl.WithCABundle(mdb.TLSCABundleSecretKeyRef(), mdb.Namespace),
		certctrl.WithCA(
			tls.ServerCASecretRef == nil,
			mdb.TLSServerCASecretKey(),
		),
		certctrl.WithCert(
			tls.ServerCertSecretRef == nil,
			mdb.TLSServerCertSecretKey(),
			mdb.TLSServerDNSNames(),
		),
		certctrl.WithCertHandler(certHandler),
		certctrl.WithCertIssuerRef(tls.ServerCertIssuerRef),
		certctrl.WithRelatedObject(mdb),
	}
	serverCertOpts = append(serverCertOpts, tlsServerCertOpts(mdb)...)

	if result, err := r.CertReconciler.Reconcile(ctx, serverCertOpts...); !result.IsZero() || err != nil {
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling server cert: %w", err)
		}
		return result.Result, nil
	}

	clientCertOpts := []certctrl.CertReconcilerOpt{
		certctrl.WithCABundle(mdb.TLSCABundleSecretKeyRef(), mdb.Namespace),
		certctrl.WithCA(
			tls.ClientCASecretRef == nil,
			mdb.TLSClientCASecretKey(),
		),
		certctrl.WithCert(
			tls.ClientCertSecretRef == nil,
			mdb.TLSClientCertSecretKey(),
			mdb.TLSClientNames(),
		),
		certctrl.WithCertHandler(certHandler),
		certctrl.WithCertIssuerRef(tls.ClientCertIssuerRef),
		certctrl.WithRelatedObject(mdb),
	}
	clientCertOpts = append(clientCertOpts, tlsClientCertOpts(mdb)...)

	if result, err := r.CertReconciler.Reconcile(ctx, clientCertOpts...); !result.IsZero() || err != nil {
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling client cert: %w", err)
		}
		return result.Result, nil
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

func (r *MariaDBReconciler) reconcileTLSConfig(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	configMapKeyRef := mariadb.TLSConfigMapKeyRef()

	tpl := createTpl("tls", `[mariadb]
ssl_cert = {{ .SSLCert }}
ssl_key = {{ .SSLKey }}
ssl_ca = {{ .SSLCA }}
require_secure_transport = {{ .RequireSecureTransport }}
tls_version = TLSv1.3
`)
	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, struct {
		SSLCert                string
		SSLKey                 string
		SSLCA                  string
		RequireSecureTransport bool
	}{
		SSLCert:                builderpki.ServerCertPath,
		SSLKey:                 builderpki.ServerKeyPath,
		SSLCA:                  builderpki.CACertPath,
		RequireSecureTransport: mariadb.IsTLSRequired(),
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
	annotations[metadata.TLSServerCertAnnotation] = hash.Hash(serverCert)

	configMapKeyRef := mariadb.TLSConfigMapKeyRef()
	config, err := r.RefResolver.ConfigMapKeyRef(ctx, &configMapKeyRef, mariadb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting TLS config: %v", err)
	}
	annotations[metadata.ConfigTLSAnnotation] = hash.Hash(config)

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
	annotations[metadata.TLSCAAnnotation] = hash.Hash(ca)

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
	annotations[metadata.TLSClientCertAnnotation] = hash.Hash(clientCert)

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

type certHandler struct {
	client.Client
	refResolver *refresolver.RefResolver
	mdb         *mariadbv1alpha1.MariaDB
}

func newCertHandler(client client.Client, refResolver *refresolver.RefResolver, mdb *mariadbv1alpha1.MariaDB) *certHandler {
	return &certHandler{
		Client:      client,
		refResolver: refResolver,
		mdb:         mdb,
	}
}

func (h *certHandler) ShouldRenewCert(ctx context.Context, caKeyPair *pki.KeyPair) (shouldRenew bool, reason string, err error) {
	if !h.mdb.IsReady() {
		return false, "MariaDB not ready", fmt.Errorf("MariaDB not ready: %w", certctrl.ErrSkipCertRenewal)
	}

	caLeafCert, err := caKeyPair.LeafCertificate()
	if err != nil {
		return false, "", fmt.Errorf("error getting CA leaf certificate: %v", err)
	}

	caBundleBytes, err := h.refResolver.SecretKeyRef(ctx, h.mdb.TLSCABundleSecretKeyRef(), h.mdb.Namespace)
	if err != nil {
		return false, "", fmt.Errorf("error getting CA bundle: %w", err)
	}
	caCerts, err := pki.ParseCertificates([]byte(caBundleBytes))
	if err != nil {
		return false, "", fmt.Errorf("error parsing CA certs: %v", err)
	}

	serialNo := caLeafCert.SerialNumber
	hasSerialNo := false
	for _, cert := range caCerts {
		if cert.SerialNumber.Cmp(serialNo) == 0 {
			hasSerialNo = true
			break
		}
	}
	// CA bundle hasn't been updated with the CA
	if !hasSerialNo {
		return false, fmt.Sprintf("Missing CA with serial number '%s' in CA bundle", serialNo.String()), nil
	}

	allPodsTrustingCA, err := h.ensureAllPodsTrustingCABundle(ctx, h.mdb, hash.Hash(caBundleBytes))
	if err != nil {
		return false, "", fmt.Errorf("error checking pod CAs: %v", err)
	}
	// Some Pods are still not trusting the CA, a rolling upgrade is pending/ongoing
	if !allPodsTrustingCA {
		return false, "Waiting for all Pods to trust CA", nil
	}

	return true, "", nil
}

func (h *certHandler) HandleExpiredCert(ctx context.Context) error {
	if h.mdb.IsReady() {
		return nil
	}
	if h.mdb.IsGaleraEnabled() {
		if err := h.patchStatus(ctx, h.mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
			status.GaleraRecovery = nil
			condition.SetGaleraNotReady(status)
		}); err != nil {
			return fmt.Errorf("error patching MariaDB status: %v", err)
		}
	}
	return nil
}

func (h *certHandler) ensureAllPodsTrustingCABundle(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	caBundleHash string) (bool, error) {
	logger := log.FromContext(ctx).WithName("pod-ca").WithValues("ca-hash", caBundleHash)

	list := corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mdb).
				Build(),
		),
		Namespace: mdb.GetNamespace(),
	}
	if err := h.List(ctx, &list, listOpts); err != nil {
		return false, fmt.Errorf("error listing Pods: %v", err)
	}
	if len(list.Items) != int(mdb.Spec.Replicas) {
		return false, errors.New("some Pods are missing")
	}

	for _, p := range list.Items {
		if !pod.PodReady(&p) {
			logger.V(1).Info("Pod not ready", "pod", p.Name)
			return false, nil
		}

		annotations := p.ObjectMeta.Annotations
		if annotations == nil {
			return false, nil
		}
		caAnnotation, ok := annotations[metadata.TLSCAAnnotation]
		if !ok {
			logger.V(1).Info("CA annotation not present", "pod", p.Name)
			return false, nil
		}
		if caAnnotation != caBundleHash {
			logger.V(1).Info("CA annotation mistmatch", "pod", p.Name, "pod-hash", caAnnotation)
			return false, nil
		}
	}
	return true, nil
}

func (r *certHandler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
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

func tlsServerCertOpts(mdb *mariadbv1alpha1.MariaDB) []certctrl.CertReconcilerOpt {
	var opts []certctrl.CertReconcilerOpt
	// Galera not compatible with key usages
	if !mdb.IsGaleraEnabled() {
		opts = append(opts, certctrl.WithServerCertKeyUsage())
	}
	return opts
}

func tlsClientCertOpts(mdb *mariadbv1alpha1.MariaDB) []certctrl.CertReconcilerOpt {
	var opts []certctrl.CertReconcilerOpt
	// Galera not compatible with key usages
	if !mdb.IsGaleraEnabled() {
		opts = append(opts, certctrl.WithClientCertKeyUsage())
	}
	return opts
}
