package v1alpha1

import (
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pki"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// InternalServiceKey defines the key for the internal headless Service
func (m *MaxScale) InternalServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-internal", m.Name),
		Namespace: m.Namespace,
	}
}

// GuiServiceKey defines the key for the GUI Service
func (m *MaxScale) GuiServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-gui", m.Name),
		Namespace: m.Namespace,
	}
}

// ConnectionKey defines the key for the Connection
func (m *MaxScale) ConnectionKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-connection", m.Name),
		Namespace: m.Namespace,
	}
}

// AdminPasswordSecretKeyRef defines the Secret key selector for the admin password
func (m *MaxScale) AdminPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-admin", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// MetricsPasswordSecretKeyRef defines the Secret key selector for the metrics password
func (m *MaxScale) MetricsPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-metrics", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// MetricsConfigSecretKeyRef defines the key selector for the metrics Secret configuration
func (m *MaxScale) MetricsConfigSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-metrics-config", m.Name),
			},
			Key: "exporter.cnf",
		},
		Generate: true,
	}
}

// MetricsKey defines the key for the metrics related resources
func (m *MaxScale) MetricsKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-metrics", m.Name),
		Namespace: m.Namespace,
	}
}

// ConfigSecretKeyRef defines the Secret key selector for the configuration
func (m *MaxScale) ConfigSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-config", m.Name),
			},
			Key: "maxscale.cnf",
		},
		Generate: true,
	}
}

// TLSCABundleSecretKeyRef defines the key selector for the TLS Secret trust bundle
func (m *MaxScale) TLSCABundleSecretKeyRef() SecretKeySelector {
	return SecretKeySelector{
		LocalObjectReference: LocalObjectReference{
			Name: fmt.Sprintf("%s-ca-bundle", m.Name),
		},
		Key: pki.CACertKey,
	}
}

// TLSServerCASecretKey defines the key for the TLS admin CA.
func (m *MaxScale) TLSAdminCASecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, MaxScaleTLS{})
	if tls.Enabled {
		if tls.AdminCASecretRef != nil {
			return types.NamespacedName{
				Name:      tls.AdminCASecretRef.Name,
				Namespace: m.Namespace,
			}
		}
		if tls.AdminCertIssuerRef != nil {
			// Secret issued by cert-manager containing the ca.crt field.
			return types.NamespacedName{
				Name:      m.TLSAdminCertSecretKey().Name,
				Namespace: m.Namespace,
			}
		}
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-ca", m.Name),
		Namespace: m.Namespace,
	}
}

// TLSAdminCertSecretKey defines the key for the TLS admin cert.
func (m *MaxScale) TLSAdminCertSecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, MaxScaleTLS{})
	if tls.Enabled && tls.AdminCertSecretRef != nil {
		return types.NamespacedName{
			Name:      tls.AdminCertSecretRef.Name,
			Namespace: m.Namespace,
		}
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-admin-cert", m.Name),
		Namespace: m.Namespace,
	}
}

// TLSListenerCASecretKey defines the key for the TLS listener CA.
func (m *MaxScale) TLSListenerCASecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, MaxScaleTLS{})
	if tls.Enabled {
		if tls.ListenerCASecretRef != nil {
			return types.NamespacedName{
				Name:      tls.ListenerCASecretRef.Name,
				Namespace: m.Namespace,
			}
		}
		if tls.ListenerCertIssuerRef != nil {
			// Secret issued by cert-manager containing the ca.crt field.
			return types.NamespacedName{
				Name:      m.TLSListenerCertSecretKey().Name,
				Namespace: m.Namespace,
			}
		}
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-ca", m.Name),
		Namespace: m.Namespace,
	}
}

// TLSListenerCertSecretKey defines the key for the TLS listener cert.
func (m *MaxScale) TLSListenerCertSecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, MaxScaleTLS{})
	if tls.Enabled && tls.ListenerCertSecretRef != nil {
		return types.NamespacedName{
			Name:      tls.ListenerCertSecretRef.Name,
			Namespace: m.Namespace,
		}
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-listener-cert", m.Name),
		Namespace: m.Namespace,
	}
}

// TLSServerCASecretKey defines the key for the TLS MariaDB server CA.
func (m *MaxScale) TLSServerCASecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, MaxScaleTLS{})
	if tls.Enabled && tls.ServerCASecretRef != nil {
		return types.NamespacedName{
			Name:      tls.ServerCASecretRef.Name,
			Namespace: m.Namespace,
		}
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-ca", m.Name),
		Namespace: m.Namespace,
	}
}

// TLSServerCertSecretKey defines the key for the TLS MariaDB server cert.
func (m *MaxScale) TLSServerCertSecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, MaxScaleTLS{})
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

// AuthClientUserKey defines the key for the client User
func (m *MaxScale) AuthClientUserKey() LocalObjectReference {
	return LocalObjectReference{
		Name: fmt.Sprintf("%s-client", m.Name),
	}
}

// AuthClientPasswordSecretKeyRef defines the Secret key selector for the client password
func (m *MaxScale) AuthClientPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-client", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// AuthClientUserKey defines the key for the monitor User
func (m *MaxScale) AuthServerUserKey() LocalObjectReference {
	return LocalObjectReference{
		Name: fmt.Sprintf("%s-server", m.Name),
	}
}

// AuthClientPasswordSecretKeyRef defines the Secret key selector for the server password
func (m *MaxScale) AuthServerPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-server", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// AuthClientUserKey defines the key for the monitor User
func (m *MaxScale) AuthMonitorUserKey() LocalObjectReference {
	return LocalObjectReference{
		Name: fmt.Sprintf("%s-monitor", m.Name),
	}
}

// AuthClientPasswordSecretKeyRef defines the Secret key selector for the monitor password
func (m *MaxScale) AuthMonitorPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-monitor", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// AuthSyncUserKey defines the key for the config sync User
func (m *MaxScale) AuthSyncUserKey() LocalObjectReference {
	return LocalObjectReference{
		Name: fmt.Sprintf("%s-sync", m.Name),
	}
}

// AuthSyncPasswordSecretKeyRef defines the Secret key selector for the config sync password
func (m *MaxScale) AuthSyncPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-sync", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}
