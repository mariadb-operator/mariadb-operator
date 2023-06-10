package builder

import (
	"fmt"
	"strconv"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	defaultStsProbe = corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"-c",
					"mysql -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
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

		container.Name = "init-galera"
		container.Image = mariadb.Spec.Galera.InitContainer.Image.String()
		container.Command = []string{"sh", "-c"}
		container.Args = []string{
			fmt.Sprintf("%s/%s", galeraresources.GaleraConfigMapMountPath, galeraresources.GaleraInitScript),
		}
		container.VolumeMounts = buildGaleraVolumeMounts(mariadb)

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

	if mariadb.Spec.Galera != nil {
		containers = append(containers, buildGaleraAgentContainer(mariadb))
	}
	if mariadb.Spec.Metrics != nil {
		if dsn == nil {
			return nil, fmt.Errorf("DSN secret is mandatory when MariaDB specifies metrics")
		}
		containers = append(containers, buildMetricsContainer(mariadb.Spec.Metrics, dsn))
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

func buildGaleraVolumeMounts(mariadb *mariadbv1alpha1.MariaDB) []corev1.VolumeMount {
	volumeMounts := buildStsVolumeMounts(mariadb)
	return append(volumeMounts, corev1.VolumeMount{
		Name:      galeraresources.GaleraConfigMapVolume,
		MountPath: galeraresources.GaleraConfigMapMountPath,
	})
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
				Name:          galeraresources.GaleraClusterPortName,
				ContainerPort: galeraresources.GaleraClusterPort,
			},
			{
				Name:          galeraresources.GaleraISTPortName,
				ContainerPort: galeraresources.GaleraISTPort,
			},
			{
				Name:          galeraresources.GaleraSSTPortName,
				ContainerPort: galeraresources.GaleraSSTPort,
			},
		}...)
	}
	return ports
}

func buildGaleraAgentContainer(mariadb *mariadbv1alpha1.MariaDB) corev1.Container {
	container := buildContainer(&mariadb.Spec.Galera.Agent.ContainerTemplate)
	container.Name = AgentContainerName
	container.Ports = []corev1.ContainerPort{
		{
			Name:          galeraresources.AgentPortName,
			ContainerPort: mariadb.Spec.Galera.Agent.Port,
		},
	}
	container.Args = []string{
		fmt.Sprintf("--addr=:%d", mariadb.Spec.Galera.Agent.Port),
		fmt.Sprintf("--config-dir=%s", galeraresources.GaleraConfigMountPath),
		fmt.Sprintf("--recovery-retries=%d", mariadb.Spec.Galera.Agent.RecoveryRetries),
		fmt.Sprintf("--recovery-retry-wait=%s", mariadb.Spec.Galera.Agent.RecoveryRetryWait.Duration.String()),
	}
	container.VolumeMounts = buildGaleraVolumeMounts(mariadb)
	container.LivenessProbe = func() *corev1.Probe {
		if container.LivenessProbe != nil {
			return container.LivenessProbe
		}
		return defaultAgentProbe(mariadb.Spec.Galera)
	}()
	container.ReadinessProbe = func() *corev1.Probe {
		if container.ReadinessProbe != nil {
			return container.ReadinessProbe
		}
		return defaultAgentProbe(mariadb.Spec.Galera)
	}()
	container.SecurityContext = func() *corev1.SecurityContext {
		if container.SecurityContext != nil {
			return container.SecurityContext
		}
		runAsUser := int64(0)
		return &corev1.SecurityContext{
			RunAsUser: &runAsUser,
		}
	}()

	return container
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
						"mysql -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SHOW STATUS LIKE 'wsrep_ready'\" | grep -c ON",
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

func defaultAgentProbe(galera *mariadbv1alpha1.Galera) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/health",
				Port: intstr.FromInt(int(galera.Agent.Port)),
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
	}
}
