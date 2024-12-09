package controller

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MaxScaleReconciler) reconcileTLS(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if !req.mxs.IsTLSEnabled() {
		return ctrl.Result{}, nil
	}
	if err := r.reconcileTLSCABundle(ctx, req.mxs); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileTLSCABundle(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) error {
	logger := log.FromContext(ctx).WithName("ca-bundle")

	caBundleKeySelector := mxs.TLSCABundleSecretKeyRef()
	adminCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSAdminCASecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	listenerCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSListenerCASecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	serverCAKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSServerCASecretKey().Name,
		},
		Key: pki.TLSCertKey,
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
		annotations[annotation] = hash(cert)
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
	annotations[metadata.TLSCAAnnotation] = hash(ca)

	adminCertKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: mxs.TLSAdminCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	clientCert, err := r.RefResolver.SecretKeyRef(ctx, adminCertKeySelector, mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting admin cert: %v", err)
	}
	annotations[metadata.TLSClientCertAnnotation] = hash(clientCert)

	return annotations, nil
}
