package builder

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	MetricsPortName        = "metrics"
	metricsConfigVolume    = "config"
	metricsConfigMountPath = "/etc/cnf/"
)

func (b *Builder) BuildExporterDeployment(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) (*appsv1.Deployment, error) {
	if !mariadb.AreMetricsEnabled() {
		return nil, errors.New("MariaDB instance does not specify Metrics")
	}
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMetricsSelectorLabels(mariadb).
			Build()
	container, err := buildExporterContainer(mariadb)
	if err != nil {
		return nil, fmt.Errorf("error building exporter container: %v", err)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: objMeta,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: selectorLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						container,
					},
					Volumes: []corev1.Volume{
						{
							Name: metricsConfigVolume,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: mariadb.MetricsConfigSecretKeyRef().Name,
								},
							},
						},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, deployment, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Deployment: %v", err)
	}
	return deployment, nil
}

func buildExporterContainer(mariadb *mariadbv1alpha1.MariaDB) (corev1.Container, error) {
	if mariadb.Spec.Metrics == nil {
		return corev1.Container{}, errors.New("metrics should be set")
	}
	metrics := *mariadb.Spec.Metrics
	tpl := metrics.Exporter.ContainerTemplate

	container := buildContainer(metrics.Exporter.Image, metrics.Exporter.ImagePullPolicy, &tpl)
	container.Name = "exporter"
	container.Args = []string{
		fmt.Sprintf("--config.my-cnf=%s", exporterConfigFile(mariadb)),
	}
	if len(tpl.Args) > 0 {
		container.Args = append(container.Args, tpl.Args...)
	}
	container.Ports = []corev1.ContainerPort{
		{
			Name:          MetricsPortName,
			ContainerPort: metrics.Exporter.Port,
		},
	}
	container.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: metricsConfigMountPath,
			ReadOnly:  true,
		},
	}

	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/",
				Port: intstr.FromInt(int(metrics.Exporter.Port)),
			},
		},
	}
	container.LivenessProbe = probe
	container.ReadinessProbe = probe

	return container, nil
}

func exporterConfigFile(mariadb *mariadbv1alpha1.MariaDB) string {
	return fmt.Sprintf("%s/%s", metricsConfigMountPath, mariadb.MetricsConfigSecretKeyRef().Key)
}
