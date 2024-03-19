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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	MetricsPortName                = "metrics"
	deployConfigVolume             = "config"
	deployConfigMountPath          = "/etc/config/"
	maxScaleRuntimeConfigVolume    = "runtime-config"
	maxScaleRuntimeConfigMountPath = "/var/lib/maxscale/maxscale.cnf.d"
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

	podTemplate, err := b.exporterPodTemplate(mariadb, key, selectorLabels)
	if err != nil {
		return nil, fmt.Errorf("error building exporter pod template: %v", err)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: objMeta,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: *podTemplate,
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, deployment, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Deployment: %v", err)
	}
	return deployment, nil
}

func (b *Builder) exporterPodTemplate(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName,
	selectorLabels map[string]string) (*corev1.PodTemplateSpec, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			WithLabels(selectorLabels).
			Build()
	container, err := exporterContainer(mariadb)
	if err != nil {
		return nil, fmt.Errorf("error building exporter container: %v", err)
	}
	exporter := ptr.Deref(mariadb.Spec.Metrics, mariadbv1alpha1.MariadbMetrics{}).Exporter
	affinity := ptr.Deref(exporter.Affinity, mariadbv1alpha1.AffinityConfig{}).Affinity

	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			ImagePullSecrets: exporterImagePullSecrets(mariadb, exporter.ImagePullSecrets),
			Containers: []corev1.Container{
				container,
			},
			Volumes: []corev1.Volume{
				{
					Name: deployConfigVolume,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: mariadb.MetricsConfigSecretKeyRef().Name,
						},
					},
				},
			},
			SecurityContext:           exporter.PodSecurityContext,
			Affinity:                  &affinity,
			NodeSelector:              exporter.NodeSelector,
			Tolerations:               exporter.Tolerations,
			PriorityClassName:         priorityClass(exporter.PriorityClassName),
			TopologySpreadConstraints: exporter.TopologySpreadConstraints,
		},
	}, nil
}

func exporterContainer(mariadb *mariadbv1alpha1.MariaDB) (corev1.Container, error) {
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
			Name:      deployConfigVolume,
			MountPath: deployConfigMountPath,
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
	return fmt.Sprintf("%s/%s", deployConfigMountPath, mariadb.MetricsConfigSecretKeyRef().Key)
}

func exporterImagePullSecrets(mariadb *mariadbv1alpha1.MariaDB, pullSecrets []corev1.LocalObjectReference) []corev1.LocalObjectReference {
	var secrets []corev1.LocalObjectReference
	secrets = append(secrets, mariadb.Spec.ImagePullSecrets...)
	secrets = append(secrets, pullSecrets...)
	return secrets
}
