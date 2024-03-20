package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// InternalServiceKey defines the key for the internal headless Service
func (m *MaxScale) InternalServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-internal", m.Name),
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
func (m *MaxScale) AdminPasswordSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-admin", m.Name),
		},
		Key: "password",
	}
}

// MetricsPasswordSecretKeyRef defines the Secret key selector for the metrics password
func (m *MaxScale) MetricsPasswordSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-metrics", m.Name),
		},
		Key: "password",
	}
}

// MetricsConfigSecretKeyRef defines the key selector for the metrics Secret configuration
func (m *MaxScale) MetricsConfigSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-metrics-config", m.Name),
		},
		Key: "exporter.cnf",
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
func (m *MaxScale) ConfigSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-config", m.Name),
		},
		Key: "maxscale.cnf",
	}
}

// AuthClientUserKey defines the key for the client User
func (m *MaxScale) AuthClientUserKey() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: fmt.Sprintf("%s-client", m.Name),
	}
}

// AuthClientPasswordSecretKeyRef defines the Secret key selector for the client password
func (m *MaxScale) AuthClientPasswordSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-client", m.Name),
		},
		Key: "password",
	}
}

// AuthClientUserKey defines the key for the monitor User
func (m *MaxScale) AuthServerUserKey() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: fmt.Sprintf("%s-server", m.Name),
	}
}

// AuthClientPasswordSecretKeyRef defines the Secret key selector for the server password
func (m *MaxScale) AuthServerPasswordSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-server", m.Name),
		},
		Key: "password",
	}
}

// AuthClientUserKey defines the key for the monitor User
func (m *MaxScale) AuthMonitorUserKey() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: fmt.Sprintf("%s-monitor", m.Name),
	}
}

// AuthClientPasswordSecretKeyRef defines the Secret key selector for the monitor password
func (m *MaxScale) AuthMonitorPasswordSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-monitor", m.Name),
		},
		Key: "password",
	}
}

// AuthSyncUserKey defines the key for the config sync User
func (m *MaxScale) AuthSyncUserKey() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: fmt.Sprintf("%s-sync", m.Name),
	}
}

// AuthSyncPasswordSecretKeyRef defines the Secret key selector for the config sync password
func (m *MaxScale) AuthSyncPasswordSecretKeyRef() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-sync", m.Name),
		},
		Key: "password",
	}
}
