package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// RootPasswordSecretKeyRef defines the key selector for the root password Secret.
func (m *MariaDB) RootPasswordSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-root", m.Name),
		},
		Key: "password",
	}
}

// PasswordSecretKeyRef defines the key selector for the initial user password Secret.
func (m *MariaDB) PasswordSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-password", m.Name),
		},
		Key: "password",
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
		Name:      fmt.Sprintf("%s-internal", m.Name),
		Namespace: m.Namespace,
	}
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
func (m *MariaDB) MetricsPasswordSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-metrics-password", m.Name),
		},
		Key: "password",
	}
}

// MetricsConfigSecretKeyRef defines the key selector for the metrics Secret configuration
func (m *MariaDB) MetricsConfigSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-metrics-config", m.Name),
		},
		Key: "exporter.cnf",
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
