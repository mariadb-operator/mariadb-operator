package builders

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func BuildService(mariadb *databasev1alpha1.MariaDB, key types.NamespacedName) *corev1.Service {
	labels :=
		NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			WithComponent(componentDatabase).
			Build()
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    buildPorts(mariadb),
			Selector: labels,
		},
	}
}

func buildPorts(mariadb *databasev1alpha1.MariaDB) []v1.ServicePort {
	ports := []v1.ServicePort{
		{
			Name: mariaDbPortName,
			Port: mariadb.Spec.Port,
		},
	}

	if mariadb.Spec.Metrics != nil {
		metricsPort := v1.ServicePort{
			Name: metricsPortName,
			Port: metricsPort,
		}
		ports = append(ports, metricsPort)
	}

	return ports
}
