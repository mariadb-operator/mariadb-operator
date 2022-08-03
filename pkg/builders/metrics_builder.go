package builders

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

type ExporterOpts struct {
	ContainerName string
	PortName      string
	Port          int32
	DSN           *corev1.SecretKeySelector
}

func BuildExporterContainer(exporter *databasev1alpha1.Exporter, opts ExporterOpts) v1.Container {
	container := v1.Container{
		Name:            opts.ContainerName,
		Image:           exporter.Image.String(),
		ImagePullPolicy: exporter.Image.PullPolicy,
		Ports: []v1.ContainerPort{
			{
				Name:          opts.PortName,
				ContainerPort: opts.Port,
			},
		},
		Env: []v1.EnvVar{
			{
				Name: "DATA_SOURCE_NAME",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: opts.DSN,
				},
			},
		},
	}

	if exporter.Resources != nil {
		container.Resources = *exporter.Resources
	}

	return container
}
