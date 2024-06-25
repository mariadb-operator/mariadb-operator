package builder

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var (
	MariadbContainerName = "mariadb"
	MariadbPortName      = "mariadb"

	MaxScaleContainerName = "maxscale"
	MaxScaleAdminPortName = "admin"

	InitContainerName  = "init"
	AgentContainerName = "agent"

	defaultProbe = corev1.Probe{
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
		PeriodSeconds:       5,
	}
	defaultGaleraAgentProbe = func(galera mariadbv1alpha1.Galera) *corev1.Probe {
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

func (b *Builder) mariadbContainers(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) ([]corev1.Container, error) {
	mariadbOpts := newMariadbPodOpts(opts...)
	mariadbContainer, err := b.buildContainer(mariadb.Spec.Image, mariadb.Spec.ImagePullPolicy, &mariadb.Spec.ContainerTemplate, opts...)
	if err != nil {
		return nil, err
	}

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
	containers = append(containers, *mariadbContainer)

	if mariadb.IsGaleraEnabled() && mariadbOpts.includeGalera {
		agentContainer, err := b.galeraAgentContainer(mariadb)
		if err != nil {
			return nil, err
		}
		containers = append(containers, *agentContainer)
	}
	if mariadb.Spec.SidecarContainers != nil {
		for index, container := range mariadb.Spec.SidecarContainers {
			sidecarContainer, err := b.buildContainer(container.Image, container.ImagePullPolicy, &container.ContainerTemplate)
			if err != nil {
				return nil, err
			}

			sidecarContainer.Name = fmt.Sprintf("sidecar-%d", index)
			if sidecarContainer.Env == nil {
				sidecarContainer.Env = mariadbEnv(mariadb)
			}
			if sidecarContainer.VolumeMounts == nil {
				sidecarContainer.VolumeMounts = mariadbVolumeMounts(mariadb)
			}
			containers = append(containers, *sidecarContainer)
		}
	}

	return containers, nil
}

func (b *Builder) maxscaleContainers(mxs *mariadbv1alpha1.MaxScale) ([]corev1.Container, error) {
	tpl := mxs.Spec.ContainerTemplate
	container, err := b.buildContainer(mxs.Spec.Image, mxs.Spec.ImagePullPolicy, &tpl)
	if err != nil {
		return nil, err
	}
	command, err := b.maxscaleCommand(mxs)
	if err != nil {
		return nil, err
	}

	container.Name = MaxScaleContainerName
	container.Command = command.Command
	container.Args = command.Args
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

	return []corev1.Container{*container}, nil
}

func (b *Builder) maxscaleCommand(mxs *mariadbv1alpha1.MaxScale) (*command.Command, error) {
	sccExists, err := b.discovery.SecurityContextConstrainstsExist()
	if err != nil {
		return nil, err
	}
	if sccExists && b.discovery.IsEnterprise() {
		return command.NewBashCommand(
			[]string{
				fmt.Sprintf(
					"maxscale --config %s -dU $(id -u) -l stdout",
					fmt.Sprintf("%s/%s", MaxscaleConfigMountPath, mxs.ConfigSecretKeyRef().Key),
				),
			},
		), nil
	}
	return command.NewCommand(
		[]string{
			"maxscale",
		},
		[]string{
			"--config",
			fmt.Sprintf("%s/%s", MaxscaleConfigMountPath, mxs.ConfigSecretKeyRef().Key),
			"-dU",
			"maxscale",
			"-l",
			"stdout",
		},
	), nil
}

func (b *Builder) galeraAgentContainer(mariadb *mariadbv1alpha1.MariaDB) (*corev1.Container, error) {
	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	recovery := ptr.Deref(galera.Recovery, mariadbv1alpha1.GaleraRecovery{})
	agent := galera.Agent

	container, err := b.buildContainer(agent.Image, agent.ImagePullPolicy, &agent.ContainerTemplate)
	if err != nil {
		return nil, err
	}

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
		return defaultGaleraAgentProbe(galera)
	}()
	container.ReadinessProbe = func() *corev1.Probe {
		if container.ReadinessProbe != nil {
			return container.ReadinessProbe
		}
		return defaultGaleraAgentProbe(galera)
	}()
	return container, nil
}

func (b *Builder) mariadbInitContainers(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) ([]corev1.Container, error) {
	mariadbOpts := newMariadbPodOpts(opts...)
	initContainers := []corev1.Container{}
	if mariadb.Spec.InitContainers != nil {
		for index, container := range mariadb.Spec.InitContainers {
			initContainer, err := b.buildContainer(container.Image, container.ImagePullPolicy, &container.ContainerTemplate)
			if err != nil {
				return nil, err
			}

			initContainer.Name = fmt.Sprintf("init-%d", index)
			if initContainer.Env == nil {
				initContainer.Env = mariadbEnv(mariadb)
			}
			if initContainer.VolumeMounts == nil {
				initContainer.VolumeMounts = mariadbVolumeMounts(mariadb)
			}
			initContainers = append(initContainers, *initContainer)
		}
	}
	if mariadb.IsGaleraEnabled() && mariadbOpts.includeGalera {
		initContainer, err := b.galeraInitContainer(mariadb)
		if err != nil {
			return nil, err
		}

		initContainers = append(initContainers, *initContainer)
	}
	return initContainers, nil
}

func (b *Builder) maxscaleInitContainers(mxs *mariadbv1alpha1.MaxScale) ([]corev1.Container, error) {
	var initContainers []corev1.Container
	if mxs.Spec.InitContainers != nil {
		for index, container := range mxs.Spec.InitContainers {
			initContainer, err := b.buildContainer(container.Image, container.ImagePullPolicy, &container.ContainerTemplate)
			if err != nil {
				return nil, err
			}

			initContainer.Name = fmt.Sprintf("init-%d", index)
			if initContainer.VolumeMounts == nil {
				initContainer.VolumeMounts = maxscaleVolumeMounts(mxs)
			}
			initContainers = append(initContainers, *initContainer)
		}
	}
	return initContainers, nil
}

func (b *Builder) galeraInitContainer(mariadb *mariadbv1alpha1.MariaDB) (*corev1.Container, error) {
	if !mariadb.IsGaleraEnabled() {
		return nil, errors.New("Galera is not enabled")
	}
	init := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).InitContainer
	container, err := b.buildContainer(init.Image, init.ImagePullPolicy, &init.ContainerTemplate)
	if err != nil {
		return nil, err
	}

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

	return container, nil
}

func (b *Builder) buildContainer(image string, pullPolicy corev1.PullPolicy, tpl *mariadbv1alpha1.ContainerTemplate,
	opts ...mariadbPodOpt) (*corev1.Container, error) {
	mariadbOpts := newMariadbPodOpts(opts...)
	sc, err := b.buildContainerSecurityContext(tpl.SecurityContext)
	if err != nil {
		return nil, err
	}

	container := corev1.Container{
		Image:           image,
		ImagePullPolicy: pullPolicy,
		Command:         tpl.Command,
		Args:            tpl.Args,
		Env:             tpl.Env,
		EnvFrom:         tpl.EnvFrom,
		VolumeMounts:    tpl.VolumeMounts,
		SecurityContext: sc,
	}
	if mariadbOpts.resources != nil {
		container.Resources = *mariadbOpts.resources
	} else if tpl.Resources != nil {
		container.Resources = *tpl.Resources
	}
	return &container, nil
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
				SecretKeyRef: &mariadb.Spec.RootPasswordSecretKeyRef.SecretKeySelector,
			},
		})
	}

	if mariadb.Spec.Env != nil {
		env = append(env, mariadb.Spec.Env...)
	}

	return env
}

func mariadbVolumeMounts(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) []corev1.VolumeMount {
	mariadbOpts := newMariadbPodOpts(opts...)
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
		{
			Name:      RunVolume,
			MountPath: MaxScaleRunMountPath,
		},
		{
			Name:      LogVolume,
			MountPath: MaxScaleLogMountPath,
		},
		{
			Name:      CacheVolume,
			MountPath: MaxScaleCacheMountPath,
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

func mariadbLivenessProbe(mariadb *mariadbv1alpha1.MariaDB) *corev1.Probe {
	if mariadb.IsGaleraEnabled() {
		return mariadbGaleraProbe(mariadb, "/liveness", mariadb.Spec.LivenessProbe)
	}
	return mariadbProbe(mariadb, mariadb.Spec.LivenessProbe)
}

func mariadbReadinessProbe(mariadb *mariadbv1alpha1.MariaDB) *corev1.Probe {
	if mariadb.IsGaleraEnabled() {
		return mariadbGaleraProbe(mariadb, "/readiness", mariadb.Spec.ReadinessProbe)
	}
	return mariadbProbe(mariadb, mariadb.Spec.ReadinessProbe)
}

func mariadbProbe(mariadb *mariadbv1alpha1.MariaDB, probe *corev1.Probe) *corev1.Probe {
	if mariadb.Replication().Enabled && ptr.Deref(mariadb.Replication().ProbesEnabled, false) {
		replProbe := mariadbReplProbe(mariadb, probe)
		setProbeThresholds(replProbe, probe)
		return replProbe
	}
	if probe != nil && probe.ProbeHandler != (corev1.ProbeHandler{}) {
		return probe
	}
	defaultProbe := defaultProbe.DeepCopy()
	setProbeThresholds(defaultProbe, probe)
	return defaultProbe
}

func mariadbReplProbe(mariadb *mariadbv1alpha1.MariaDB, probe *corev1.Probe) *corev1.Probe {
	replProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"-c",
					fmt.Sprintf("%s/%s", ProbesMountPath, mariadb.ReplConfigMapKeyRef().Key),
				},
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       5,
	}
	setProbeThresholds(replProbe, probe)
	return replProbe
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
	if probe != nil && probe.ProbeHandler != (corev1.ProbeHandler{}) {
		return probe
	}
	mxsProbe := corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/",
				Port: intstr.FromInt(int(mxs.Spec.Admin.Port)),
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       5,
	}
	setProbeThresholds(&mxsProbe, probe)
	return &mxsProbe
}

func setProbeThresholds(source, target *corev1.Probe) {
	if target == nil {
		return
	}
	if target.InitialDelaySeconds > 0 {
		source.InitialDelaySeconds = target.InitialDelaySeconds
	}
	if target.TimeoutSeconds > 0 {
		source.TimeoutSeconds = target.TimeoutSeconds
	}
	if target.PeriodSeconds > 0 {
		source.PeriodSeconds = target.PeriodSeconds
	}
	if target.SuccessThreshold > 0 {
		source.SuccessThreshold = target.SuccessThreshold
	}
	if target.FailureThreshold > 0 {
		source.FailureThreshold = target.FailureThreshold
	}
}
