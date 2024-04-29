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
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
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
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
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
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
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
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: fmt.Sprintf("%s-config", m.Name),
			},
			Key: "maxscale.cnf",
		},
		Generate: true,
	}
}

// AuthClientUserKey defines the key for the client User
func (m *MaxScale) AuthClientUserKey() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: fmt.Sprintf("%s-client", m.Name),
	}
}

// AuthClientPasswordSecretKeyRef defines the Secret key selector for the client password
func (m *MaxScale) AuthClientPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: fmt.Sprintf("%s-client", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// AuthClientUserKey defines the key for the monitor User
func (m *MaxScale) AuthServerUserKey() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: fmt.Sprintf("%s-server", m.Name),
	}
}

// AuthClientPasswordSecretKeyRef defines the Secret key selector for the server password
func (m *MaxScale) AuthServerPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: fmt.Sprintf("%s-server", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// AuthClientUserKey defines the key for the monitor User
func (m *MaxScale) AuthMonitorUserKey() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: fmt.Sprintf("%s-monitor", m.Name),
	}
}

// AuthClientPasswordSecretKeyRef defines the Secret key selector for the monitor password
func (m *MaxScale) AuthMonitorPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: fmt.Sprintf("%s-monitor", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

// AuthSyncUserKey defines the key for the config sync User
func (m *MaxScale) AuthSyncUserKey() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: fmt.Sprintf("%s-sync", m.Name),
	}
}

// AuthSyncPasswordSecretKeyRef defines the Secret key selector for the config sync password
func (m *MaxScale) AuthSyncPasswordSecretKeyRef() GeneratedSecretKeyRef {
	return GeneratedSecretKeyRef{
		SecretKeySelector: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: fmt.Sprintf("%s-sync", m.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}
