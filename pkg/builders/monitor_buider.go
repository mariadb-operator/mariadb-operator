package builders

import (
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	metricsPortName = "metrics"
)

func BuildExporterDeployment(mariadb *databasev1alpha1.MariaDB, monitor *databasev1alpha1.MonitorMariaDB,
	dsn *corev1.SecretKeySelector) (*appsv1.Deployment, error) {
	containers, err := buildExporterContainers(monitor, dsn)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-exporter", mariadb.Name)
	labels := getExporterLabels(mariadb)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mariadb.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: mariadb.Namespace,
					Labels:    labels,
				},
				Spec: v1.PodSpec{
					Containers: containers,
				},
			},
		},
	}, nil
}

func BuildPodMonitor(mariadb *databasev1alpha1.MariaDB, monitor *databasev1alpha1.MonitorMariaDB) *monitoringv1.PodMonitor {
	labels :=
		NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			WithRelease(monitor.Spec.PrometheusRelease).
			Build()
	exporterLabels := getExporterLabels(mariadb)
	return &monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.Name,
			Namespace: mariadb.Namespace,
			Labels:    labels,
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: exporterLabels,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{mariadb.Namespace},
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
				{
					Port:          metricsPortName,
					Path:          "/metrics",
					Scheme:        "http",
					Interval:      monitoringv1.Duration(monitor.Spec.Interval),
					ScrapeTimeout: monitoringv1.Duration(monitor.Spec.ScrapeTimeout),
				},
			},
		},
	}
}

func getExporterLabels(mariadb *databasev1alpha1.MariaDB) map[string]string {
	return NewLabelsBuilder().
		WithApp(appMariaDb).
		WithInstance(mariadb.Name).
		WithComponent(componentExporter).
		Build()
}

func buildExporterContainers(monitor *databasev1alpha1.MonitorMariaDB, dsn *corev1.SecretKeySelector) ([]v1.Container, error) {
	container := v1.Container{
		Name:            monitor.Name,
		Image:           monitor.Spec.Exporter.Image.String(),
		ImagePullPolicy: monitor.Spec.Exporter.Image.PullPolicy,
		Ports: []v1.ContainerPort{
			{
				Name:          metricsPortName,
				ContainerPort: 9104,
			},
		},
		Env: []v1.EnvVar{
			{
				Name: "DATA_SOURCE_NAME",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: dsn,
				},
			},
		},
	}

	if monitor.Spec.Exporter.Resources != nil {
		container.Resources = *monitor.Spec.Exporter.Resources
	}

	return []v1.Container{container}, nil
}
