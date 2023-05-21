package builder

import (
	"fmt"
	"strconv"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	corev1 "k8s.io/api/core/v1"
)

var (
	defaultProbe = corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
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
)

func buildStsInitContainers(mariadb *mariadbv1alpha1.MariaDB) []corev1.Container {
	if mariadb.Spec.Galera != nil {
		volumeMounts := buildStsVolumeMounts(mariadb)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      galeraresources.GaleraConfigMapVolume,
			MountPath: galeraresources.GaleraConfigMapMountPath,
		})
		return []corev1.Container{
			{
				Name:    "init-galera",
				Image:   mariadb.Spec.Galera.InitContainerImage.String(),
				Command: []string{"sh", "-c"},
				Args: []string{
					fmt.Sprintf("%s/%s", galeraresources.GaleraConfigMapMountPath, galeraresources.GaleraInitScript),
				},
				VolumeMounts: volumeMounts,
			},
		}
	}
	return nil
}

func buildStsContainers(mariadb *mariadbv1alpha1.MariaDB, dsn *corev1.SecretKeySelector) ([]corev1.Container, error) {
	var containers []corev1.Container
	mariaDbContainer := corev1.Container{
		Name:            MariaDbContainerName,
		Image:           mariadb.Spec.Image.String(),
		ImagePullPolicy: mariadb.Spec.Image.PullPolicy,
		Args:            buildStsArgs(mariadb),
		Env:             buildStsEnv(mariadb),
		EnvFrom:         mariadb.Spec.EnvFrom,
		Ports:           buildStsPorts(mariadb),
		VolumeMounts:    buildStsVolumeMounts(mariadb),
		ReadinessProbe: func() *corev1.Probe {
			if mariadb.Spec.ReadinessProbe != nil {
				return mariadb.Spec.ReadinessProbe
			}
			return &defaultProbe
		}(),
		LivenessProbe:   buildStsLivenessProbe(mariadb),
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

func buildStsArgs(mariadb *mariadbv1alpha1.MariaDB) []string {
	if mariadb.Spec.Replication != nil {
		return []string{
			"--log-bin",
			"--log-basename",
			mariadb.Name,
		}
	}
	return nil
}

func buildStsEnv(mariadb *mariadbv1alpha1.MariaDB) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "MYSQL_TCP_PORT",
			Value: strconv.Itoa(int(mariadb.Spec.Port)),
		},
		{
			Name: "MARIADB_ROOT_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mariadb.Spec.RootPasswordSecretKeyRef,
			},
		},
		{
			Name:  "MARIADB_ROOT_HOST",
			Value: "%",
		},
		{
			Name:  "MYSQL_INITDB_SKIP_TZINFO",
			Value: "1",
		},
	}

	if mariadb.Spec.Replication == nil {
		if mariadb.Spec.Database != nil {
			env = append(env, corev1.EnvVar{
				Name:  "MARIADB_DATABASE",
				Value: *mariadb.Spec.Database,
			})
		}
		if mariadb.Spec.Username != nil {
			env = append(env, corev1.EnvVar{
				Name:  "MARIADB_USER",
				Value: *mariadb.Spec.Username,
			})
		}
		if mariadb.Spec.PasswordSecretKeyRef != nil {
			env = append(env, corev1.EnvVar{
				Name: "MARIADB_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
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

func buildStsVolumeMounts(mariadb *mariadbv1alpha1.MariaDB) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      StorageVolume,
			MountPath: StorageMountPath,
		},
		{
			Name:      ConfigVolume,
			MountPath: ConfigMountPath,
		},
	}
	if mariadb.Spec.Galera != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      galeraresources.GaleraConfigVolume,
			MountPath: galeraresources.GaleraConfigMountPath,
		})
	}
	if mariadb.Spec.VolumeMounts != nil {
		volumeMounts = append(volumeMounts, mariadb.Spec.VolumeMounts...)
	}
	return volumeMounts
}

func buildStsPorts(mariadb *mariadbv1alpha1.MariaDB) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          MariaDbPortName,
			ContainerPort: mariadb.Spec.Port,
		},
	}
	if mariadb.Spec.Galera != nil {
		ports = append(ports, []corev1.ContainerPort{
			{
				Name:          "cluster",
				ContainerPort: galeraresources.GaleraClusterPort,
			},
			{
				Name:          "ist",
				ContainerPort: galeraresources.GaleraISTPort,
			},
			{
				Name:          "sst",
				ContainerPort: galeraresources.GaleraSSTPort,
			},
		}...)
	}
	return ports
}

func buildStsLivenessProbe(mariadb *mariadbv1alpha1.MariaDB) *corev1.Probe {
	if mariadb.Spec.LivenessProbe != nil {
		return mariadb.Spec.LivenessProbe
	}
	if mariadb.Spec.Galera != nil {
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"bash",
						"-c",
						"mysql -u root -p${MARIADB_ROOT_PASSWORD} -e \"SHOW STATUS LIKE 'wsrep_ready'\" | grep -c ON",
					},
				},
			},
			InitialDelaySeconds: 60,
			TimeoutSeconds:      5,
			PeriodSeconds:       10,
		}
	}
	return &defaultProbe
}

func buildMetricsContainer(metrics *mariadbv1alpha1.Metrics, dsn *corev1.SecretKeySelector) corev1.Container {
	container := corev1.Container{
		Name:            "metrics",
		Image:           metrics.Exporter.Image.String(),
		ImagePullPolicy: metrics.Exporter.Image.PullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: 9104,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name: "DATA_SOURCE_NAME",
				ValueFrom: &corev1.EnvVarSource{
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
