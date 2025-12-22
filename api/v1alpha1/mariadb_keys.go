package v1alpha1

import (
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pki"
	stsobj "github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// RootPasswordSecretKeyRef defines the key selector for the root password Secret.
func (m *MariaDB) RootPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-root", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// PasswordSecretKeyRef defines the key selector for the initial user password Secret.
func (m *MariaDB) PasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-password", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// ReplPasswordSecretKeyRef defines the key selector for for the password to be used by the replication "repl" user
func (m *MariaDB) ReplPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-repl-password", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// DefaultConfigMapKeyRef defines the key selector for the default my.cnf ConfigMap.
func (m *MariaDB) DefaultConfigMapKeyRef() ConfigMapKeySelector {
	return ConfigMapKeySelector{
		LocalObjectReference: LocalObjectReference{
			Name: fmt.Sprintf("%s-config-default", m.Name),
		},
		Key: "0-default.cnf",
	}
}

// MyCnfConfigMapKeyRef defines the key selector for the my.cnf ConfigMap.
func (m *MariaDB) MyCnfConfigMapKeyRef() ConfigMapKeySelector {
	return ConfigMapKeySelector{
		LocalObjectReference: LocalObjectReference{
			Name: fmt.Sprintf("%s-config", m.Name),
		},
		Key: "my.cnf",
	}
}

// TLSCABundleSecretKeyRef defines the key selector for the TLS Secret trust bundle
func (m *MariaDB) TLSCABundleSecretKeyRef() SecretKeySelector {
	return SecretKeySelector{
		LocalObjectReference: LocalObjectReference{
			Name: fmt.Sprintf("%s-ca-bundle", m.Name),
		},
		Key: pki.CACertKey,
	}
}

// TLSConfigMapKeyRef defines the key selector for the TLS ConfigMap
func (m *MariaDB) TLSConfigMapKeyRef() ConfigMapKeySelector {
	return ConfigMapKeySelector{
		LocalObjectReference: LocalObjectReference{
			Name: fmt.Sprintf("%s-config-tls", m.Name),
		},
		Key: "1-tls.cnf",
	}
}

// TLSServerCASecretKey defines the key for the TLS server CA.
func (m *MariaDB) TLSServerCASecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, TLS{})
	if tls.Enabled {
		if tls.ServerCASecretRef != nil {
			return types.NamespacedName{
				Name:      tls.ServerCASecretRef.Name,
				Namespace: m.Namespace,
			}
		}
		if tls.ServerCertIssuerRef != nil {
			// Secret issued by cert-manager containing the ca.crt field.
			return types.NamespacedName{
				Name:      m.TLSServerCertSecretKey().Name,
				Namespace: m.Namespace,
			}
		}
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-ca", m.Name),
		Namespace: m.Namespace,
	}
}

// TLSServerCertSecretKey defines the key for the TLS server cert.
func (m *MariaDB) TLSServerCertSecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, TLS{})
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

// TLSClientCASecretKey defines the key for the TLS client CA.
func (m *MariaDB) TLSClientCASecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, TLS{})
	if tls.Enabled {
		if tls.ClientCASecretRef != nil {
			return types.NamespacedName{
				Name:      tls.ClientCASecretRef.Name,
				Namespace: m.Namespace,
			}
		}
		if tls.ClientCertIssuerRef != nil {
			// Secret issued by cert-manager containing the ca.crt field.
			return types.NamespacedName{
				Name:      m.TLSClientCertSecretKey().Name,
				Namespace: m.Namespace,
			}
		}
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-ca", m.Name),
		Namespace: m.Namespace,
	}
}

// TLSClientCertSecretKey defines the key for the TLS client cert.
func (m *MariaDB) TLSClientCertSecretKey() types.NamespacedName {
	tls := ptr.Deref(m.Spec.TLS, TLS{})
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

// RestoreKey defines the key for the Restore resource used to bootstrap.
func (m *MariaDB) RestoreKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-restore", m.Name),
		Namespace: m.Namespace,
	}
}

// InternalServiceKey defines the key for the internal headless Service
func (m *MariaDB) InternalServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      InternalServiceName(m.Name),
		Namespace: m.Namespace,
	}
}

// InternalServiceName defines the name for the internal headless Service
func InternalServiceName(mariadbName string) string {
	return fmt.Sprintf("%s-internal", mariadbName)
}

// PrimaryServiceKey defines the key for the primary Service
func (m *MariaDB) PrimaryServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-primary", m.Name),
		Namespace: m.Namespace,
	}
}

// PrimaryConnectioneKey defines the key for the primary Connection
func (m *MariaDB) PrimaryConnectioneKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-primary", m.Name),
		Namespace: m.Namespace,
	}
}

// SecondaryServiceKey defines the key for the secondary Service
func (m *MariaDB) SecondaryServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-secondary", m.Name),
		Namespace: m.Namespace,
	}
}

// SecondaryConnectioneKey defines the key for the secondary Connection
func (m *MariaDB) SecondaryConnectioneKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-secondary", m.Name),
		Namespace: m.Namespace,
	}
}

// MetricsKey defines the key for the metrics related resources
func (m *MariaDB) MetricsKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-metrics", m.Name),
		Namespace: m.Namespace,
	}
}

// MaxScaleKey defines the key for the MaxScale resource.
func (m *MariaDB) MaxScaleKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-maxscale", m.Name),
		Namespace: m.Namespace,
	}
}

// MetricsPasswordSecretKeyRef defines the key selector for for the password to be used by the metrics user
func (m *MariaDB) MetricsPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-metrics-password", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// MetricsConfigSecretKeyRef defines the key selector for the metrics Secret configuration
func (m *MariaDB) MetricsConfigSecretKeyRef() GeneratedSecretKeyRef {
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

// InitKey defines the keys for the init objects.
func (m *MariaDB) InitKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-init", m.Name),
		Namespace: m.Namespace,
	}
}

// PhysicalBackupInitJobKey defines the keys for the PhysicalBackup init Job objects.
func (m *MariaDB) PhysicalBackupInitJobKey(podIndex int) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-pb-init", stsobj.PodName(m.ObjectMeta, podIndex)),
		Namespace: m.Namespace,
	}
}

// PhysicalBackupStagingPVCKey defines the key for the PhysicalBackup staging PVC object.
func (m *MariaDB) PhysicalBackupStagingPVCKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-pb-staging", m.Name),
		Namespace: m.Namespace,
	}
}

// PhysicalBackupScaleOutKey defines the key for the PhysicalBackup scale out object.
func (m *MariaDB) PhysicalBackupScaleOutKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-pb-scaleout", m.Name),
		Namespace: m.Namespace,
	}
}

// PhysicalBackupScaleOutKey defines the key for the PhysicalBackup replica recovery object.
func (m *MariaDB) PhysicalBackupReplicaRecoveryKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-pb-recovery", m.Name),
		Namespace: m.Namespace,
	}
}

// RecoveryJobKey defines the key for a Galera recovery Job
func (m *MariaDB) RecoveryJobKey(podName string) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-recovery", podName),
		Namespace: m.Namespace,
	}
}

// PVCKey defines the PVC keys.
func (m *MariaDB) PVCKey(name string, index int) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s-%d", name, m.Name, index),
		Namespace: m.Namespace,
	}
}

// MariadbSysUserKey defines the key for the 'mariadb.sys' User resource.
func (m *MariaDB) MariadbSysUserKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-mariadb-sys", m.Name),
		Namespace: m.Namespace,
	}
}

// MariadbSysGrantKey defines the key for the 'mariadb.sys' Grant resource.
func (m *MariaDB) MariadbSysGrantKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-mariadb-sys-global-priv", m.Name),
		Namespace: m.Namespace,
	}
}

// MariadbDatabaseKey defines the key for the initial database
func (m *MariaDB) MariadbDatabaseKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-database", m.Name),
		Namespace: m.Namespace,
	}
}

// MariadbUserKey defines the key for the initial user
func (m *MariaDB) MariadbUserKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-user", m.Name),
		Namespace: m.Namespace,
	}
}

// MariadbGrantKey defines the key for the initial grant
func (m *MariaDB) MariadbGrantKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-user-all", m.Name),
		Namespace: m.Namespace,
	}
}

// AgentAuthSecretKeyRef defines the Secret key selector for the agent password
func (m *MariaDB) AgentAuthSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: SecretKeySelector{
			LocalObjectReference: LocalObjectReference{
				Name: fmt.Sprintf("%s-agent-auth", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}
