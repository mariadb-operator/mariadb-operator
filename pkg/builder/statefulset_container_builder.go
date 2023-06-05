package builder

import (
	"fmt"
	"strconv"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	corev1 "k8s.io/api/core/v1"
)

var (
	defaultStsProbe = corev1.Probe{
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
		container := buildContainer(&mariadb.Spec.Galera.InitContainer)
		volumeMounts := buildStsVolumeMounts(mariadb)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      galeraresources.GaleraConfigMapVolume,
			MountPath: galeraresources.GaleraConfigMapMountPath,
		})

		container.Name = "init-galera"
		container.Image = mariadb.Spec.Galera.InitContainer.Image.String()
		container.Command = []string{"sh", "-c"}
		container.Args = []string{
			fmt.Sprintf("%s/%s", galeraresources.GaleraConfigMapMountPath, galeraresources.GaleraInitScript),
		}
		container.VolumeMounts = volumeMounts

		return []corev1.Container{
			container,
		}
	}
	return nil
}

func buildStsContainers(mariadb *mariadbv1alpha1.MariaDB, dsn *corev1.SecretKeySelector) ([]corev1.Container, error) {
	mariadbContainer := buildContainer(&mariadb.Spec.ContainerTemplate)
	mariadbContainer.Name = MariaDbContainerName
	mariadbContainer.Args = buildStsArgs(mariadb)
	mariadbContainer.Env = buildStsEnv(mariadb)
	mariadbContainer.Ports = buildStsPorts(mariadb)
	mariadbContainer.VolumeMounts = buildStsVolumeMounts(mariadb)
	mariadbContainer.LivenessProbe = buildStsLivenessProbe(mariadb)
	mariadbContainer.ReadinessProbe = func() *corev1.Probe {
		if mariadbContainer.ReadinessProbe != nil {
			return mariadbContainer.ReadinessProbe
		}
		return &defaultStsProbe
	}()

	var containers []corev1.Container
	containers = append(containers, mariadbContainer)

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
	return &defaultStsProbe
}

func buildMetricsContainer(metrics *mariadbv1alpha1.Metrics, dsn *corev1.SecretKeySelector) corev1.Container {
	container := buildContainer(&metrics.Exporter.ContainerTemplate)
	container.Name = MetricsContainerName
	container.Ports = []corev1.ContainerPort{
		{
			Name:          MetricsPortName,
			ContainerPort: metrics.Exporter.Port,
		},
	}
	container.Env = append(container.Env, corev1.EnvVar{
		Name: "DATA_SOURCE_NAME",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: dsn,
		},
	})
	return container
}

func buildContainer(tpl *mariadbv1alpha1.ContainerTemplate) corev1.Container {
	container := corev1.Container{
		Image:           tpl.Image.String(),
		ImagePullPolicy: tpl.Image.PullPolicy,
		Env:             tpl.Env,
		EnvFrom:         tpl.EnvFrom,
		VolumeMounts:    tpl.VolumeMounts,
		LivenessProbe:   tpl.LivenessProbe,
		ReadinessProbe:  tpl.ReadinessProbe,
		SecurityContext: tpl.SecurityContext,
	}
	if tpl.Resources != nil {
		container.Resources = *tpl.Resources
	}
	return container
}
