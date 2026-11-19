package controller

import (
	"context"
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	certctrl "github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/certificate"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileClientCertificate issues a dedicated TLS client certificate for the User (spec.clientCertificate.enabled),
// signed by the same CA used by the referenced MariaDB. This lets a User authenticate to MariaDB with its own
// X.509 client certificate instead of a password.
func (r *UserReconciler) reconcileClientCertificate(ctx context.Context, user *mariadbv1alpha1.User) (ctrl.Result, error) {
	if !user.IsClientCertificateEnabled() {
		return ctrl.Result{}, nil
	}
	log.FromContext(ctx).V(1).Info("Reconciling User client certificate")

	mdb, err := r.RefResolver.MariaDB(ctx, &user.Spec.MariaDBRef, user.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf(
			"error getting MariaDB: %w ('spec.clientCertificate' requires 'spec.mariaDbRef' to point to a MariaDB resource)", err,
		)
	}
	if !mdb.IsTLSEnabled() {
		return ctrl.Result{}, errors.New("'spec.clientCertificate' requires the referenced MariaDB to have 'spec.tls.enabled' set to true")
	}

	if user.ClientCertCollidesWith(mdb) {
		return ctrl.Result{}, fmt.Errorf(
			"'spec.clientCertificate' is not allowed: its certificate (CN=%s, Secret %s) clashes with the MariaDB's own client "+
				"certificate (CN=%s, Secret %s); use a different User name",
			user.TLSClientNames()[0], user.TLSClientCertSecretKey().Name, mdb.TLSClientNames()[0], mdb.TLSClientCertSecretKey().Name,
		)
	}

	tls := mdb.Spec.TLS
	certOpts := []certctrl.CertReconcilerOpt{
		certctrl.WithCABundle(mdb.TLSCABundleSecretKeyRef(), mdb.Namespace),
		certctrl.WithCA(false, mdb.TLSClientCASecretKey()),
		certctrl.WithCert(true, user.TLSClientCertSecretKey(), user.TLSClientNames()),
		certctrl.WithClientCertKeyUsage(),
		certctrl.WithCertIssuerRef(tls.ClientCertIssuerRef),
		certctrl.WithRelatedObject(user),
	}

	result, err := r.CertReconciler.Reconcile(ctx, certOpts...)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling client cert: %w", err)
	}
	return result.Result, nil
}
