package builders

import (
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildExporterDeployment(mariadb *databasev1alpha1.MariaDB, monitor *databasev1alpha1.MonitorMariaDB,
	dsn *corev1.SecretKeySelector) (*appsv1.Deployment, error) {
	containers, err := buildExporterContainers(monitor, dsn)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-exporter", mariadb.Name)
	labels :=
		NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			WithComponent(componentExporter).
			Build()
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

func buildExporterContainers(monitor *databasev1alpha1.MonitorMariaDB, dsn *corev1.SecretKeySelector) ([]v1.Container, error) {
	container := v1.Container{
		Name:            monitor.Name,
		Image:           monitor.Spec.Exporter.Image.String(),
		ImagePullPolicy: monitor.Spec.Exporter.Image.PullPolicy,
		Ports: []v1.ContainerPort{
			{
				Name:          "metrics",
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
