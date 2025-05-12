package controller

import (
	"context"
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	certctrl "github.com/mariadb-operator/mariadb-operator/pkg/controller/certificate"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/hash"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MaxScaleReconciler) reconcileTLS(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if !req.mxs.IsTLSEnabled() {
		return ctrl.Result{}, nil
	}
	if err := r.reconcileTLSCerts(ctx, req.mxs); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileTLSCABundle(ctx, req.mxs); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileTLSCerts(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) error {
	tls := ptr.Deref(mxs.Spec.TLS, mariadbv1alpha1.MaxScaleTLS{})

	// MaxScale TLS can't communicate with a non-TLS MariaDB server
	if tls.ServerCASecretRef == nil || tls.ServerCertSecretRef == nil {
		return errors.New("'spec.tls.serverCASecretRef' and 'spec.tls.ServerCertSecretRef' fields" +
			"must be set in order to communicate with MariaDB server")
	}

	adminCertOpts := []certctrl.CertReconcilerOpt{
		certctrl.WithCABundle(mxs.TLSCABundleSecretKeyRef(), mxs.Namespace),
		certctrl.WithCA(
			tls.AdminCASecretRef == nil,
			mxs.TLSAdminCASecretKey(),
		),
		certctrl.WithCert(
			tls.AdminCertSecretRef == nil,
			mxs.TLSAdminCertSecretKey(),
			mxs.TLSAdminDNSNames(),
		),
		certctrl.WithServerCertKeyUsage(),
		certctrl.WithCertIssuerRef(tls.AdminCertIssuerRef),
		certctrl.WithRelatedObject(mxs),
	}
	if _, err := r.CertReconciler.Reconcile(ctx, adminCertOpts...); err != nil {
		return fmt.Errorf("error reconciling admin cert: %v", err)
	}

	listenerCertOpts := []certctrl.CertReconcilerOpt{
		certctrl.WithCABundle(mxs.TLSCABundleSecretKeyRef(), mxs.Namespace),
		certctrl.WithCA(
			tls.ListenerCASecretRef == nil,
			mxs.TLSAdminCASecretKey(),
		),
		certctrl.WithCert(
			tls.ListenerCertSecretRef == nil,
			mxs.TLSListenerCertSecretKey(),
			mxs.TLSListenerDNSNames(),
		),
		certctrl.WithServerCertKeyUsage(),
		certctrl.WithCertIssuerRef(tls.ListenerCertIssuerRef),
		certctrl.WithRelatedObject(mxs),
	}
	if _, err := r.CertReconciler.Reconcile(ctx, listenerCertOpts...); err != nil {
		return fmt.Errorf("error reconciling listener cert: %v", err)
	}

	return nil
}

func (r *MaxScaleReconciler) reconcileTLSCABundle(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) error {
	logger := log.FromContext(ctx).WithName("ca-bundle")

	caBundleKeySelector := mxs.TLSCABundleSecretKeyRef()
	adminCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSAdminCASecretKey().Name,
		},
		Key: pki.CACertKey,
	}
	listenerCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSListenerCASecretKey().Name,
		},
		Key: pki.CACertKey,
	}
	serverCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSServerCASecretKey().Name,
		},
		Key: pki.CACertKey,
	}
	caKeySelectors := []mariadbv1alpha1.SecretKeySelector{
		caBundleKeySelector,
		adminCAKeySelector,
		listenerCAKeySelector,
		serverCAKeySelector,
	}
	var caBundles [][]byte

	for _, caKeySelector := range caKeySelectors {
		ca, err := r.RefResolver.SecretKeyRef(ctx, caKeySelector, mxs.Namespace)
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
		Metadata: []*mariadbv1alpha1.Metadata{mxs.Spec.InheritMetadata},
		Owner:    mxs,
		Key: types.NamespacedName{
			Name:      caBundleKeySelector.Name,
			Namespace: mxs.Namespace,
		},
		Data: map[string][]byte{
			caBundleKeySelector.Key: bundle,
		},
	}
	return r.SecretReconciler.Reconcile(ctx, &secretReq)
}

func (r *MaxScaleReconciler) getTLSAnnotations(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (map[string]string, error) {
	if !mxs.IsTLSEnabled() {
		return nil, nil
	}
	annotations, err := r.getTLSAdminAnnotations(ctx, mxs)
	if err != nil {
		return nil, fmt.Errorf("error getting client annotations: %v", err)
	}

	secretSelectorsByAnn := map[string]mariadbv1alpha1.SecretKeySelector{
		metadata.TLSListenerCertAnnotation: {
			LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
				Name: mxs.TLSListenerCertSecretKey().Name,
			},
			Key: pki.TLSCertKey,
		},
		metadata.TLSServerCertAnnotation: {
			LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
				Name: mxs.TLSServerCertSecretKey().Name,
			},
			Key: pki.TLSCertKey,
		},
	}

	for annotation, secretKeySelector := range secretSelectorsByAnn {
		cert, err := r.RefResolver.SecretKeyRef(ctx, secretKeySelector, mxs.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting Secret \"%s\": %v", secretKeySelector.Name, err)
		}
		annotations[annotation] = hash.Hash(cert)
	}

	return annotations, nil
}

func (r *MaxScaleReconciler) getTLSAdminAnnotations(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (map[string]string, error) {
	if !mxs.IsTLSEnabled() {
		return nil, nil
	}
	annotations := make(map[string]string)

	ca, err := r.RefResolver.SecretKeyRef(ctx, mxs.TLSCABundleSecretKeyRef(), mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting CA bundle: %v", err)
	}
	annotations[metadata.TLSCAAnnotation] = hash.Hash(ca)

	adminCertKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSAdminCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	adminCert, err := r.RefResolver.SecretKeyRef(ctx, adminCertKeySelector, mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting admin cert: %v", err)
	}
	annotations[metadata.TLSAdminCertAnnotation] = hash.Hash(adminCert)

	return annotations, nil
}

func (r *MaxScaleReconciler) getTLSStatus(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (*mariadbv1alpha1.MaxScaleTLSStatus, error) {
	if !mxs.IsTLSEnabled() {
		return nil, nil
	}
	var tlsStatus mariadbv1alpha1.MaxScaleTLSStatus

	certStatus, err := getCertificateStatus(ctx, r.RefResolver, mxs.TLSCABundleSecretKeyRef(), mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting CA bundle status: %v", err)
	}
	tlsStatus.CABundle = certStatus

	secretKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSAdminCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	certStatus, err = getCertificateStatus(ctx, r.RefResolver, secretKeySelector, mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting admin certificate status: %v", err)
	}
	tlsStatus.AdminCert = ptr.To(certStatus[0])

	secretKeySelector = mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSListenerCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	certStatus, err = getCertificateStatus(ctx, r.RefResolver, secretKeySelector, mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting listener certificate status: %v", err)
	}
	tlsStatus.ListenerCert = ptr.To(certStatus[0])

	secretKeySelector = mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSServerCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	certStatus, err = getCertificateStatus(ctx, r.RefResolver, secretKeySelector, mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting server certificate status: %v", err)
	}
	tlsStatus.ServerCert = ptr.To(certStatus[0])

	return &tlsStatus, nil
}
