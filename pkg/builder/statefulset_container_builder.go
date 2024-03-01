package builder

import (
	"fmt"
	"os"
	"strconv"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func (b *Builder) mariadbContainers(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbOpt) ([]corev1.Container, error) {
	mariadbOpts := newMariadbOpts(opts...)
	mariadbContainer := buildContainer(mariadb.Spec.Image, mariadb.Spec.ImagePullPolicy, &mariadb.Spec.ContainerTemplate)
	mariadbContainer.Name = MariadbContainerName
	mariadbContainer.Env = mariadbEnv(mariadb)
	mariadbContainer.VolumeMounts = mariadbVolumeMounts(mariadb, opts...)

	if mariadbOpts.includePorts {
		mariadbContainer.Ports = mariadbPorts(mariadb)
	}
	if mariadbOpts.includeProbes {
		mariadbContainer.LivenessProbe = mariadbLivenessProbe(mariadb)
		mariadbContainer.ReadinessProbe = mariadbReadinessProbe(mariadb)
	}

	if mariadbOpts.command != nil {
		mariadbContainer.Command = mariadbOpts.command
	}
	if mariadbOpts.args != nil {
		mariadbContainer.Args = mariadbOpts.args
	} else {
		mariadbContainer.Args = mariadbArgs(mariadb)
	}

	var containers []corev1.Container
	containers = append(containers, mariadbContainer)

	if mariadb.IsGaleraEnabled() && mariadbOpts.includeGalera {
		containers = append(containers, b.galeraAgentContainer(mariadb))
	}
	if mariadb.Spec.SidecarContainers != nil {
		for index, container := range mariadb.Spec.SidecarContainers {
			sidecarContainer := buildContainer(container.Image, container.ImagePullPolicy, &container.ContainerTemplate)
			sidecarContainer.Name = fmt.Sprintf("sidecar-%d", index)
			if sidecarContainer.Env == nil {
				sidecarContainer.Env = mariadbEnv(mariadb)
			}
			if sidecarContainer.VolumeMounts == nil {
				sidecarContainer.VolumeMounts = mariadbVolumeMounts(mariadb)
			}
			containers = append(containers, sidecarContainer)
		}
	}

	return containers, nil
}

func (b *Builder) maxscaleContainers(mxs *mariadbv1alpha1.MaxScale) ([]corev1.Container, error) {
	tpl := mxs.Spec.ContainerTemplate

	container := buildContainer(mxs.Spec.Image, mxs.Spec.ImagePullPolicy, &tpl)
	container.Name = MaxScaleContainerName
	container.Command = []string{
		"maxscale",
	}
	container.Args = []string{
		"--config",
		fmt.Sprintf("%s/%s", MaxscaleConfigMountPath, mxs.ConfigSecretKeyRef().Key),
		"-dU",
		"maxscale",
		"-l",
		"stdout",
	}
	if len(tpl.Args) > 0 {
		container.Args = append(container.Args, tpl.Args...)
	}
	container.Ports = []corev1.ContainerPort{
		{
			Name:          MaxScaleAdminPortName,
			ContainerPort: int32(mxs.Spec.Admin.Port),
		},
	}
	container.VolumeMounts = maxscaleVolumeMounts(mxs)
	container.LivenessProbe = maxscaleProbe(mxs, mxs.Spec.LivenessProbe)
	container.ReadinessProbe = maxscaleProbe(mxs, mxs.Spec.ReadinessProbe)

	return []corev1.Container{container}, nil
}

func (b *Builder) galeraAgentContainer(mariadb *mariadbv1alpha1.MariaDB) corev1.Container {
	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	agent := galera.Agent
	recovery := galera.Recovery

	container := buildContainer(agent.Image, agent.ImagePullPolicy, &agent.ContainerTemplate)
	container.Name = AgentContainerName
	container.Ports = []corev1.ContainerPort{
		{
			Name:          galeraresources.AgentPortName,
			ContainerPort: agent.Port,
		},
	}
	container.Args = func() []string {
		args := container.Args
		args = append(args, []string{
			"agent",
			fmt.Sprintf("--addr=:%d", agent.Port),
			fmt.Sprintf("--config-dir=%s", galeraresources.GaleraConfigMountPath),
			fmt.Sprintf("--state-dir=%s", MariadbStorageMountPath),
		}...)
		if agent.GracefulShutdownTimeout != nil {
			args = append(args, fmt.Sprintf("--graceful-shutdown-timeout=%s", agent.GracefulShutdownTimeout.Duration))
		}
		if recovery.Enabled && recovery.PodRecoveryTimeout != nil {
			args = append(args, fmt.Sprintf("--recovery-timeout=%s", recovery.PodRecoveryTimeout.Duration))
		}
		if ptr.Deref(agent.KubernetesAuth, mariadbv1alpha1.KubernetesAuth{}).Enabled {
			args = append(args, []string{
				"--kubernetes-auth",
				fmt.Sprintf("--kubernetes-trusted-name=%s", b.env.MariadbOperatorName),
				fmt.Sprintf("--kubernetes-trusted-namespace=%s", b.env.MariadbOperatorNamespace),
			}...)
		}
		return args
	}()
	container.Env = mariadbEnv(mariadb)
	container.VolumeMounts = mariadbVolumeMounts(mariadb)
	container.LivenessProbe = func() *corev1.Probe {
		if container.LivenessProbe != nil {
			return container.LivenessProbe
		}
		return defaultAgentProbe(galera)
	}()
	container.ReadinessProbe = func() *corev1.Probe {
		if container.ReadinessProbe != nil {
			return container.ReadinessProbe
		}
		return defaultAgentProbe(galera)
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

func mariadbInitContainers(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbOpt) []corev1.Container {
	mariadbOpts := newMariadbOpts(opts...)
	initContainers := []corev1.Container{}
	if mariadb.Spec.InitContainers != nil {
		for index, container := range mariadb.Spec.InitContainers {
			initContainer := buildContainer(container.Image, container.ImagePullPolicy, &container.ContainerTemplate)
			initContainer.Name = fmt.Sprintf("init-%d", index)
			if initContainer.Env == nil {
				initContainer.Env = mariadbEnv(mariadb)
			}
			if initContainer.VolumeMounts == nil {
				initContainer.VolumeMounts = mariadbVolumeMounts(mariadb)
			}
			initContainers = append(initContainers, initContainer)
		}
	}
	if mariadb.IsGaleraEnabled() && mariadbOpts.includeGalera {
		initContainers = append(initContainers, galeraInitContainer(mariadb))
	}
	return initContainers
}

func maxscaleInitContainers(mxs *mariadbv1alpha1.MaxScale) []corev1.Container {
	initContainers := []corev1.Container{
		{
			Name:  "init-chown",
			Image: mxs.Spec.Image,
			Command: []string{
				"/bin/sh",
				"-c",
				"chown -R 998:996 /var/lib/maxscale",
			},
			VolumeMounts: maxscaleVolumeMounts(mxs),
			SecurityContext: &corev1.SecurityContext{
				RunAsUser: ptr.To(int64(0)),
			},
		},
	}
	if mxs.Spec.InitContainers != nil {
		for index, container := range mxs.Spec.InitContainers {
			initContainer := buildContainer(container.Image, container.ImagePullPolicy, &container.ContainerTemplate)
			initContainer.Name = fmt.Sprintf("init-%d", index)
			if initContainer.VolumeMounts == nil {
				initContainer.VolumeMounts = maxscaleVolumeMounts(mxs)
			}
			initContainers = append(initContainers, initContainer)
		}
	}
	return initContainers
}

func galeraInitContainer(mariadb *mariadbv1alpha1.MariaDB) corev1.Container {
	if !mariadb.IsGaleraEnabled() {
		return corev1.Container{}
	}
	init := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).InitContainer
	container := buildContainer(init.Image, init.ImagePullPolicy, &init.ContainerTemplate)

	container.Name = InitContainerName
	container.Args = func() []string {
		args := container.Args
		args = append(args, []string{
			"init",
			fmt.Sprintf("--config-dir=%s", galeraresources.GaleraConfigMountPath),
			fmt.Sprintf("--state-dir=%s", MariadbStorageMountPath),
		}...)
		return args
	}()
	container.Env = mariadbEnv(mariadb)
	container.VolumeMounts = mariadbVolumeMounts(mariadb)

	return container
}

func mariadbArgs(mariadb *mariadbv1alpha1.MariaDB) []string {
	if mariadb.Replication().Enabled {
		return []string{
			"--log-bin",
			fmt.Sprintf("--log-basename=%s", mariadb.Name),
		}
	}
	return nil
}

func mariadbEnv(mariadb *mariadbv1alpha1.MariaDB) []corev1.EnvVar {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "cluster.local"
	}

	env := []corev1.EnvVar{
		{
			Name:  "MYSQL_TCP_PORT",
			Value: strconv.Itoa(int(mariadb.Spec.Port)),
		},
		{
			Name:  "MARIADB_ROOT_HOST",
			Value: "%",
		},
		{
			Name:  "MYSQL_INITDB_SKIP_TZINFO",
			Value: "1",
		},
		{
			Name:  "CLUSTER_NAME",
			Value: clusterName,
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name:  "MARIADB_NAME",
			Value: mariadb.Name,
		},
	}

	if mariadb.IsRootPasswordEmpty() {
		env = append(env, corev1.EnvVar{
			Name:  "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD",
			Value: "yes",
		})
	} else {
		env = append(env, corev1.EnvVar{
			Name: "MARIADB_ROOT_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mariadb.Spec.RootPasswordSecretKeyRef,
			},
		})
	}

	if !mariadb.Replication().Enabled {
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

func mariadbVolumeMounts(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbOpt) []corev1.VolumeMount {
	mariadbOpts := newMariadbOpts(opts...)
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      ConfigVolume,
			MountPath: MariadbConfigMountPath,
		},
	}
	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	reuseStorageVolume := ptr.Deref(galera.Config.ReuseStorageVolume, false)

	storageVolumeMount := corev1.VolumeMount{
		Name:      StorageVolume,
		MountPath: MariadbStorageMountPath,
	}
	if mariadb.IsGaleraEnabled() && reuseStorageVolume {
		storageVolumeMount.SubPath = StorageVolume
	}
	volumeMounts = append(volumeMounts, storageVolumeMount)

	if mariadb.Replication().Enabled && ptr.Deref(mariadb.Replication().ProbesEnabled, false) {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ProbesVolume,
			MountPath: ProbesMountPath,
		})
	}
	if mariadb.IsGaleraEnabled() && mariadbOpts.includeGalera {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ServiceAccountVolume,
			MountPath: ServiceAccountMountPath,
		})

		galeraConfigVolumeMount := corev1.VolumeMount{
			MountPath: galeraresources.GaleraConfigMountPath,
		}
		if reuseStorageVolume {
			galeraConfigVolumeMount.Name = StorageVolume
			galeraConfigVolumeMount.SubPath = galeraresources.GaleraConfigVolume
		} else {
			galeraConfigVolumeMount.Name = galeraresources.GaleraConfigVolume
		}

		volumeMounts = append(volumeMounts, galeraConfigVolumeMount)
	}
	if mariadb.Spec.VolumeMounts != nil {
		volumeMounts = append(volumeMounts, mariadb.Spec.VolumeMounts...)
	}
	if mariadbOpts.extraVolumeMounts != nil {
		volumeMounts = append(volumeMounts, mariadbOpts.extraVolumeMounts...)
	}
	return volumeMounts
}

func maxscaleVolumeMounts(maxscale *mariadbv1alpha1.MaxScale) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      StorageVolume,
			MountPath: MaxscaleStorageMountPath,
		},
		{
			Name:      ConfigVolume,
			MountPath: MaxscaleConfigMountPath,
		},
	}
	if maxscale.Spec.VolumeMounts != nil {
		volumeMounts = append(volumeMounts, maxscale.Spec.VolumeMounts...)
	}
	return volumeMounts
}

func mariadbPorts(mariadb *mariadbv1alpha1.MariaDB) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          MariadbPortName,
			ContainerPort: mariadb.Spec.Port,
		},
	}
	if mariadb.IsGaleraEnabled() {
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

func buildContainer(image string, pullPolicy corev1.PullPolicy, tpl *mariadbv1alpha1.ContainerTemplate) corev1.Container {
	container := corev1.Container{
		Image:           image,
		ImagePullPolicy: pullPolicy,
		Command:         tpl.Command,
		Args:            tpl.Args,
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

func mariadbLivenessProbe(mariadb *mariadbv1alpha1.MariaDB) *corev1.Probe {
	if mariadb.IsGaleraEnabled() {
		return mariadbGaleraProbe(mariadb, "/liveness", mariadb.Spec.LivenessProbe)
	}
	return mariadbProbe(mariadb, mariadb.Spec.LivenessProbe)
}

func mariadbReadinessProbe(mariadb *mariadbv1alpha1.MariaDB) *corev1.Probe {
	if mariadb.IsGaleraEnabled() {
		return mariadbGaleraProbe(mariadb, "/readiness", mariadb.Spec.LivenessProbe)
	}
	return mariadbProbe(mariadb, mariadb.Spec.ReadinessProbe)
}

func mariadbProbe(mariadb *mariadbv1alpha1.MariaDB, probe *corev1.Probe) *corev1.Probe {
	if mariadb.Replication().Enabled && ptr.Deref(mariadb.Replication().ProbesEnabled, false) {
		replProbe := mariadbReplProbe(mariadb, probe)
		setProbeThresholds(replProbe, probe)
		return replProbe
	}
	if probe != nil {
		return probe
	}
	return &defaultStsProbe
}

func mariadbReplProbe(mariadb *mariadbv1alpha1.MariaDB, probe *corev1.Probe) *corev1.Probe {
	mxsProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"-c",
					fmt.Sprintf("%s/%s", ProbesMountPath, mariadb.ReplConfigMapKeyRef().Key),
				},
			},
		},
		InitialDelaySeconds: 40,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
	}
	setProbeThresholds(mxsProbe, probe)
	return mxsProbe
}

func mariadbGaleraProbe(mdb *mariadbv1alpha1.MariaDB, path string, probe *corev1.Probe) *corev1.Probe {
	agent := ptr.Deref(mdb.Spec.Galera, mariadbv1alpha1.Galera{}).Agent
	galeraProbe := corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt(int(agent.Port)),
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       5,
	}
	setProbeThresholds(&galeraProbe, probe)
	return &galeraProbe
}

func maxscaleProbe(mxs *mariadbv1alpha1.MaxScale, probe *corev1.Probe) *corev1.Probe {
	if probe != nil {
		return probe
	}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/",
				Port: intstr.FromInt(int(mxs.Spec.Admin.Port)),
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
	}
}

func setProbeThresholds(source, target *corev1.Probe) {
	if target == nil {
		return
	}
	source.InitialDelaySeconds = target.InitialDelaySeconds
	source.TimeoutSeconds = target.TimeoutSeconds
	source.PeriodSeconds = target.PeriodSeconds
	source.SuccessThreshold = target.SuccessThreshold
	source.FailureThreshold = target.FailureThreshold
}

var (
	defaultStsProbe = corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"-c",
					"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
				},
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
	}
	defaultAgentProbe = func(galera mariadbv1alpha1.Galera) *corev1.Probe {
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(int(galera.Agent.Port)),
				},
			},
		}
	}
)
