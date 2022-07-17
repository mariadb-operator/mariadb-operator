package builders

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	metricsPortName = "metrics"
)

func BuildExporterMariaDB(mariadb *databasev1alpha1.MariaDB, exporter *databasev1alpha1.Exporter,
	key types.NamespacedName) *databasev1alpha1.ExporterMariaDB {
	labels := getExporterLabels(mariadb)
	return &databasev1alpha1.ExporterMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Spec: databasev1alpha1.ExporterMariaDBSpec{
			MariaDBRef: corev1.LocalObjectReference{
				Name: mariadb.Name,
			},
			Exporter: *exporter,
		},
	}
}

func BuildExporterDeployment(mariadb *databasev1alpha1.MariaDB, exporter *databasev1alpha1.ExporterMariaDB,
	key types.NamespacedName, dsn *corev1.SecretKeySelector) (*appsv1.Deployment, error) {
	containers, err := buildExporterContainers(exporter, dsn)
	if err != nil {
		return nil, err
	}
	labels := getExporterLabels(mariadb)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
					Labels:    labels,
				},
				Spec: v1.PodSpec{
					Containers: containers,
				},
			},
		},
	}, nil
}

func BuildPodMonitor(mariadb *databasev1alpha1.MariaDB, monitor *databasev1alpha1.MonitorMariaDB,
	key types.NamespacedName) *monitoringv1.PodMonitor {
	labels :=
		NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			WithRelease(monitor.Spec.PrometheusRelease).
			Build()
	exporterLabels := getExporterLabels(mariadb)
	return &monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
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

func buildExporterContainers(exporter *databasev1alpha1.ExporterMariaDB, dsn *corev1.SecretKeySelector) ([]v1.Container, error) {
	container := v1.Container{
		Name:            exporter.Name,
		Image:           exporter.Spec.Exporter.Image.String(),
		ImagePullPolicy: exporter.Spec.Exporter.Image.PullPolicy,
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

	if exporter.Spec.Exporter.Resources != nil {
		container.Resources = *exporter.Spec.Exporter.Resources
	}

	return []v1.Container{container}, nil
}
