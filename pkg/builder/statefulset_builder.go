package builder

import (
	"errors"
	"fmt"
	"strconv"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	stsStorageVolume    = "storage"
	stsStorageMountPath = "/var/lib/mysql"
	stsConfigVolume     = "config"
	stsConfigMountPath  = "/etc/mysql/conf.d"

	mariaDbContainerName = "mariadb"
	mariaDbPortName      = "mariadb"

	metricsContainerName = "metrics"
	metricsPortName      = "metrics"
	metricsPort          = 9104
)

func PVCKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s-0", stsStorageVolume, mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func StatefulSetPort(sts *appsv1.StatefulSet) (*corev1.ContainerPort, error) {
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == mariaDbContainerName {
			for _, p := range c.Ports {
				if p.Name == mariaDbPortName {
					return &p, nil
				}
			}
		}
	}
	return nil, errors.New("StatefulSet port not found")
}

func (b *Builder) BuildStatefulSet(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName,
	dsn *corev1.SecretKeySelector) (*appsv1.StatefulSet, error) {
	containers, err := buildStatefulSetContainers(mariadb, dsn)
	if err != nil {
		return nil, fmt.Errorf("error building MariaDB containers: %v", err)
	}

	statefulSetLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			Build()
	pvcMeta := metav1.ObjectMeta{
		Name:      stsStorageVolume,
		Namespace: mariadb.Namespace,
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    statefulSetLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         mariadb.Name,
			Replicas:            &mariadb.Spec.Replicas,
			PodManagementPolicy: buildStatefulSetPodManagementPolicy(mariadb),
			Selector: &metav1.LabelSelector{
				MatchLabels: statefulSetLabels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mariadb.Name,
					Namespace: mariadb.Namespace,
					Labels:    statefulSetLabels,
				},
				Spec: v1.PodSpec{
					Containers:      containers,
					Volumes:         buildStatefulSetVolumes(mariadb),
					SecurityContext: mariadb.Spec.PodSecurityContext,
					Affinity:        mariadb.Spec.Affinity,
					NodeSelector:    mariadb.Spec.NodeSelector,
					Tolerations:     mariadb.Spec.Tolerations,
				},
			},
			VolumeClaimTemplates: []v1.PersistentVolumeClaim{
				corev1.PersistentVolumeClaim{
					ObjectMeta: pvcMeta,
					Spec:       mariadb.Spec.VolumeClaimTemplate,
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, sts, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to StatefulSet: %v", err)
	}
	return sts, nil
}

func buildStatefulSetContainers(mariadb *mariadbv1alpha1.MariaDB, dsn *corev1.SecretKeySelector) ([]v1.Container, error) {
	var containers []v1.Container
	defaultProbe := &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			Exec: &v1.ExecAction{
				Command: []string{
					"bash",
					"-c",
					"mysql -u root -p${MARIADB_ROOT_PASSWORD} -e \"SELECT 1;\"",
				},
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
	}
	mariaDbContainer := v1.Container{
		Name:            mariaDbContainerName,
		Image:           mariadb.Spec.Image.String(),
		ImagePullPolicy: mariadb.Spec.Image.PullPolicy,
		Args:            buildStatefulSetArgs(mariadb),
		Env:             buildStatefulSetEnv(mariadb),
		EnvFrom:         mariadb.Spec.EnvFrom,
		Ports: []v1.ContainerPort{
			{
				Name:          mariaDbPortName,
				ContainerPort: mariadb.Spec.Port,
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      stsStorageVolume,
				MountPath: stsStorageMountPath,
			},
		},
		ReadinessProbe: func() *corev1.Probe {
			if mariadb.Spec.ReadinessProbe != nil {
				return mariadb.Spec.ReadinessProbe
			}
			return defaultProbe
		}(),
		LivenessProbe: func() *corev1.Probe {
			if mariadb.Spec.LivenessProbe != nil {
				return mariadb.Spec.LivenessProbe
			}
			return defaultProbe
		}(),
		SecurityContext: mariadb.Spec.SecurityContext,
	}

	if mariadb.Spec.Resources != nil {
		mariaDbContainer.Resources = *mariadb.Spec.Resources
	}
	containers = append(containers, mariaDbContainer)

	if mariadb.Spec.Metrics != nil {
		if dsn == nil {
			return nil, fmt.Errorf("DSN secret is mandatory when MariaDB specifies metrics")
		}

		metricsContainer := buildMetricsContainer(mariadb.Spec.Metrics, dsn)
		containers = append(containers, metricsContainer)
	}

	return containers, nil
}

func buildStatefulSetPodManagementPolicy(mariadb *mariadbv1alpha1.MariaDB) appsv1.PodManagementPolicyType {
	if mariadb.Spec.Replication != nil {
		return appsv1.ParallelPodManagement
	}
	return appsv1.OrderedReadyPodManagement
}

func buildStatefulSetArgs(mariadb *mariadbv1alpha1.MariaDB) []string {
	if mariadb.Spec.Replication != nil {
		return []string{
			"--log-bin",
			"--log-basename",
			mariadb.Name,
		}
	}
	return nil
}

func buildStatefulSetEnv(mariadb *mariadbv1alpha1.MariaDB) []v1.EnvVar {
	env := []v1.EnvVar{
		{
			Name:  "MYSQL_TCP_PORT",
			Value: strconv.Itoa(int(mariadb.Spec.Port)),
		},
		{
			Name: "MARIADB_ROOT_PASSWORD",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &mariadb.Spec.RootPasswordSecretKeyRef,
			},
		},
		{
			Name:  "MYSQL_INITDB_SKIP_TZINFO",
			Value: "1",
		},
	}

	if mariadb.Spec.Replication == nil {
		if mariadb.Spec.Database != nil {
			env = append(env, v1.EnvVar{
				Name:  "MARIADB_DATABASE",
				Value: *mariadb.Spec.Database,
			})
		}
		if mariadb.Spec.Username != nil {
			env = append(env, v1.EnvVar{
				Name:  "MARIADB_USER",
				Value: *mariadb.Spec.Username,
			})
		}
		if mariadb.Spec.PasswordSecretKeyRef != nil {
			env = append(env, v1.EnvVar{
				Name: "MARIADB_PASSWORD",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: mariadb.Spec.PasswordSecretKeyRef,
				},
			})
		}
	}

	if mariadb.Spec.Env != nil {
		env = append(env, mariadb.Spec.Env...)
	}

	return env
}

func buildStatefulSetVolumes(mariadb *mariadbv1alpha1.MariaDB) []v1.Volume {
	volume := v1.Volume{
		Name: stsConfigVolume,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	if mariadb.Spec.MyCnfConfigMapKeyRef != nil {
		volume = v1.Volume{
			Name: stsConfigVolume,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: mariadb.Spec.MyCnfConfigMapKeyRef.Name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  mariadb.Spec.MyCnfConfigMapKeyRef.Key,
							Path: "my.cnf",
						},
					},
				},
			},
		}
	}
	return []v1.Volume{
		volume,
	}
}

func buildMetricsContainer(metrics *mariadbv1alpha1.Metrics, dsn *corev1.SecretKeySelector) v1.Container {
	container := v1.Container{
		Name:            metricsContainerName,
		Image:           metrics.Exporter.Image.String(),
		ImagePullPolicy: metrics.Exporter.Image.PullPolicy,
		Ports: []v1.ContainerPort{
			{
				Name:          metricsPortName,
				ContainerPort: metricsPort,
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

	if metrics.Exporter.Resources != nil {
		container.Resources = *metrics.Exporter.Resources
	}

	return container
}
