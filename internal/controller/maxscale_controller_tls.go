package controller

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
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
		adminCAKeySelector,
		listenerCAKeySelector,
		serverCAKeySelector,
	}
	caBundles := make([][]byte, len(caKeySelectors))

	for i, caKeySelector := range caKeySelectors {
		ca, err := r.RefResolver.SecretKeyRef(ctx, caKeySelector, mxs.Namespace)
		if err != nil {
			return fmt.Errorf("error getting CA \"%s\": %v", caKeySelector.Name, err)
		}
		caBundles[i] = []byte(ca)
	}

	bundle, err := pki.BundleCertificatePEMs(log.FromContext(ctx), caBundles...)
	if err != nil {
		return fmt.Errorf("error creating CA bundle: %v", err)
	}

	secretKeyRef := mxs.TLSCABundleSecretKeyRef()
	secretReq := secret.SecretRequest{
		Metadata: []*mariadbv1alpha1.Metadata{mxs.Spec.InheritMetadata},
		Owner:    mxs,
		Key: types.NamespacedName{
			Name:      secretKeyRef.Name,
			Namespace: mxs.Namespace,
		},
		Data: map[string][]byte{
			secretKeyRef.Key: bundle,
		},
	}
	return r.SecretReconciler.Reconcile(ctx, &secretReq)
}
