package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// RootPasswordSecretKeyRef defines the key selector for the root password Secret.
func (m *MariaDB) RootPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
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
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: fmt.Sprintf("%s-password", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// DefaultConfigMapKeyRef defines the key selector for the default my.cnf ConfigMap.
func (m *MariaDB) DefaultConfigMapKeyRef() corev1.ConfigMapKeySelector {
	return corev1.ConfigMapKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-config-default", m.Name),
		},
		Key: "0-default.cnf",
	}
}

// MyCnfConfigMapKeyRef defines the key selector for the my.cnf ConfigMap.
func (m *MariaDB) MyCnfConfigMapKeyRef() corev1.ConfigMapKeySelector {
	return corev1.ConfigMapKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-config", m.Name),
		},
		Key: "my.cnf",
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
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
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
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: fmt.Sprintf("%s-metrics-config", m.Name),
			},
			Key: "exporter.cnf",
		},
		Generate: true,
	}
}

// ConfigMapKeySelector defines the key selector for the ConfigMap used for replication healthchecks.
func (m *MariaDB) ReplConfigMapKeyRef() corev1.ConfigMapKeySelector {
	return corev1.ConfigMapKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-probes", m.Name),
		},
		Key: "replication.sh",
	}
}

// InitKey defines the keys for the init objects.
func (m *MariaDB) InitKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-init", m.Name),
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
