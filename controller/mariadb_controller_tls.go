package controller

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *MariaDBReconciler) reconcileTLS(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsTLSEnabled() {
		return ctrl.Result{}, nil
	}
	if err := r.reconcileTLSConfig(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileTLSConfig(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	configMapKeyRef := mariadb.TLSConfigMapKeyRef()
	configMapReq := configmap.ReconcileRequest{
		Metadata: mariadb.Spec.InheritMetadata,
		Owner:    mariadb,
		Key: types.NamespacedName{
			Name:      configMapKeyRef.Name,
			Namespace: mariadb.Namespace,
		},
		Data: map[string]string{
			configMapKeyRef.Key: `[mariadb]
ssl_cert = /etc/pki/server.crt
ssl_key = /etc/pki/server.key
ssl_ca = /etc/pki/ca/client.crt
require_secure_transport = true
`,
		},
	}
	return r.ConfigMapReconciler.Reconcile(ctx, &configMapReq)
}
