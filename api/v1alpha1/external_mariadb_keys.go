package v1alpha1

import (
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pki"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// TLSCABundleSecretKeyRef defines the key selector for the TLS Secret trust bundle
func (m *ExternalMariaDB) TLSCABundleSecretKeyRef() SecretKeySelector {
	if m.Spec.TLS.ServerCASecretRef == nil {
		return SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-ca-bundle", m.Name),
			},
			Key: pki.CACertKey,
		}
	}
	return SecretKeySelector{
		LocalObjectReference: LocalObjectReference{
			Name: m.Spec.TLS.ServerCASecretRef.Name,
		},
		Key: pki.CACertKey,
	}
}

// TLSServerCertSecretKey defines the key for the TLS server cert.
func (m *ExternalMariaDB) TLSServerCertSecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, ExternalTLS{})
	if tls.Enabled && tls.ServerCertSecretRef != nil {
		return types.NamespacedName{
			Name:      tls.ServerCertSecretRef.Name,
			Namespace: m.Namespace,
		}
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-server-cert", m.Name),
		Namespace: m.Namespace,
	}
}

// TLSClientCertSecretKey defines the key for the TLS client cert.
func (m *ExternalMariaDB) TLSClientCertSecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, ExternalTLS{})
	if tls.Enabled && tls.ClientCertSecretRef != nil {
		return types.NamespacedName{
			Name:      tls.ClientCertSecretRef.Name,
			Namespace: m.Namespace,
		}
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-client-cert", m.Name),
		Namespace: m.Namespace,
	}
}
