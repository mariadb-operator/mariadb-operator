package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	annotation "github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	StorageVolume            = "storage"
	MariadbStorageMountPath  = "/var/lib/mysql"
	MaxscaleStorageMountPath = "/var/lib/maxscale"
	StorageVolumeRole        = "storage"

	ConfigVolume            = "config"
	MariadbConfigMountPath  = "/etc/mysql/conf.d"
	MaxscaleConfigMountPath = "/etc/config"
	ConfigVolumeRole        = "config"

	InitVolume        = "init"
	InitConfigPath    = "/init"
	InitLibKey        = "lib.sh"
	InitEntrypointKey = "entrypoint.sh"

	ProbesVolume    = "probes"
	ProbesMountPath = "/etc/probes"

	ServiceAccountVolume    = "serviceaccount"
	ServiceAccountMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"

	MariadbContainerName = "mariadb"
	MariadbPortName      = "mariadb"

	MaxScaleContainerName = "maxscale"
	MaxScaleAdminPortName = "admin"

	InitContainerName  = "init"
	AgentContainerName = "agent"
)

func (b *Builder) BuildMariadbStatefulSet(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) (*appsv1.StatefulSet, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			WithAnnotations(mariadbHAAnnotations(mariadb)).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDBSelectorLabels(mariadb).
			Build()
	podTemplate, err := b.mariadbPodTemplate(mariadb, selectorLabels)
	if err != nil {
		return nil, fmt.Errorf("error building pod template: %v", err)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         mariadb.InternalServiceKey().Name,
			Replicas:            &mariadb.Spec.Replicas,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy:      statefulSetUpdateStrategy(mariadb.Spec.UpdateStrategy),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template:             *podTemplate,
			VolumeClaimTemplates: mariadbVolumeClaimTemplates(mariadb),
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, sts, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to StatefulSet: %v", err)
	}
	return sts, nil
}

func (b *Builder) BuildMaxscaleStatefulSet(maxscale *mariadbv1alpha1.MaxScale, key types.NamespacedName) (*appsv1.StatefulSet, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMaxScaleSelectorLabels(maxscale).
			Build()
	podTemplate, err := b.maxscalePodTemplate(maxscale, selectorLabels)
	if err != nil {
		return nil, fmt.Errorf("error building pod template: %v", err)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         maxscale.InternalServiceKey().Name,
			Replicas:            &maxscale.Spec.Replicas,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy:      statefulSetUpdateStrategy(maxscale.Spec.UpdateStrategy),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template:             *podTemplate,
			VolumeClaimTemplates: maxscaleVolumeClaimTemplates(maxscale),
		},
	}
	if err := controllerutil.SetControllerReference(maxscale, sts, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to StatefulSet: %v", err)
	}
	return sts, nil
}

type mariadbOpts struct {
	command           []string
	args              []string
	restartPolicy     *corev1.RestartPolicy
	extraVolumes      []corev1.Volume
	extraVolumeMounts []corev1.VolumeMount
	includeGalera     bool
	includePorts      bool
	includeProbes     bool
}

func newMariadbOpts(userOpts ...mariadbOpt) *mariadbOpts {
	opts := &mariadbOpts{
		includeGalera: true,
		includePorts:  true,
		includeProbes: true,
	}
	for _, setOpt := range userOpts {
		setOpt(opts)
	}
	return opts
}

type mariadbOpt func(opts *mariadbOpts)

func withCommand(command []string) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.command = command
	}
}

func withArgs(args []string) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.args = args
	}
}

func withRestartPolicy(restartPolicy corev1.RestartPolicy) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.restartPolicy = &restartPolicy
	}
}

func withExtraVolumes(volumes []corev1.Volume) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.extraVolumes = volumes
	}
}

func withExtraVolumeMounts(volumeMounts []corev1.VolumeMount) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.extraVolumeMounts = volumeMounts
	}
}

func withGalera(includeGalera bool) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.includeGalera = includeGalera
	}
}

func withPorts(includePorts bool) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.includePorts = includePorts
	}
}

func withProbes(includeProbes bool) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.includeProbes = includeProbes
	}
}

func (b *Builder) mariadbPodTemplate(mariadb *mariadbv1alpha1.MariaDB, labels map[string]string,
	opts ...mariadbOpt) (*corev1.PodTemplateSpec, error) {
	mariadbOpts := newMariadbOpts(opts...)
	containers, err := b.mariadbContainers(mariadb, opts...)
	if err != nil {
		return nil, fmt.Errorf("error building MariaDB containers: %v", err)
	}
	objMeta :=
		metadata.NewMetadataBuilder(client.ObjectKeyFromObject(mariadb)).
			WithMariaDB(mariadb).
			WithLabels(labels).
			WithAnnotations(mariadb.Spec.PodAnnotations).
			WithAnnotations(mariadbHAAnnotations(mariadb)).
			Build()
	affinity := ptr.Deref(mariadb.Spec.Affinity, mariadbv1alpha1.AffinityConfig{}).Affinity
	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           serviceAccount(mariadb.Spec.ServiceAccountName, mariadb.Name),
			RestartPolicy:                ptr.Deref(mariadbOpts.restartPolicy, corev1.RestartPolicyAlways),
			InitContainers:               mariadbInitContainers(mariadb, opts...),
			Containers:                   containers,
			ImagePullSecrets:             mariadb.Spec.ImagePullSecrets,
			Volumes:                      mariadbVolumes(mariadb, opts...),
			SecurityContext:              mariadb.Spec.PodSecurityContext,
			Affinity:                     &affinity,
			NodeSelector:                 mariadb.Spec.NodeSelector,
			Tolerations:                  mariadb.Spec.Tolerations,
			PriorityClassName:            priorityClass(mariadb.Spec.PriorityClassName),
			TopologySpreadConstraints:    mariadb.Spec.TopologySpreadConstraints,
		},
	}, nil
}

func (b *Builder) maxscalePodTemplate(mxs *mariadbv1alpha1.MaxScale, labels map[string]string) (*corev1.PodTemplateSpec, error) {
	containers, err := b.maxscaleContainers(mxs)
	if err != nil {
		return nil, fmt.Errorf("error building MaxScale containers: %v", err)
	}
	objMeta :=
		metadata.NewMetadataBuilder(client.ObjectKeyFromObject(mxs)).
			WithLabels(labels).
			Build()
	affinity := ptr.Deref(mxs.Spec.Affinity, mariadbv1alpha1.AffinityConfig{}).Affinity
	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           serviceAccount(mxs.Spec.ServiceAccountName, mxs.Name),
			InitContainers:               maxscaleInitContainers(mxs),
			Containers:                   containers,
			ImagePullSecrets:             mxs.Spec.ImagePullSecrets,
			Volumes:                      maxscaleVolumes(mxs),
			SecurityContext:              mxs.Spec.PodSecurityContext,
			Affinity:                     &affinity,
			NodeSelector:                 mxs.Spec.NodeSelector,
			Tolerations:                  mxs.Spec.Tolerations,
			PriorityClassName:            priorityClass(mxs.Spec.PriorityClassName),
			TopologySpreadConstraints:    mxs.Spec.TopologySpreadConstraints,
		},
	}, nil
}

func statefulSetUpdateStrategy(strategy *appsv1.StatefulSetUpdateStrategy) appsv1.StatefulSetUpdateStrategy {
	if strategy != nil {
		return *strategy
	}
	return appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
	}
}

func mariadbVolumeClaimTemplates(mariadb *mariadbv1alpha1.MariaDB) []corev1.PersistentVolumeClaim {
	var pvcs []corev1.PersistentVolumeClaim

	if !mariadb.IsEphemeralStorageEnabled() {
		vctpl := ptr.Deref(mariadb.Spec.Storage.VolumeClaimTemplate, mariadbv1alpha1.VolumeClaimTemplate{})
		labels := labels.NewLabelsBuilder().
			WithLabels(vctpl.Labels).
			WithPVCRole(StorageVolumeRole).
			Build()

		pvcs = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:        StorageVolume,
					Labels:      labels,
					Annotations: vctpl.Annotations,
				},
				Spec: vctpl.PersistentVolumeClaimSpec,
			},
		}
	}

	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	reuseStorageVolume := ptr.Deref(galera.Config.ReuseStorageVolume, false)
	vctpl := galera.Config.VolumeClaimTemplate

	if mariadb.IsGaleraEnabled() && !reuseStorageVolume && vctpl != nil {
		labels := labels.NewLabelsBuilder().
			WithLabels(vctpl.Labels).
			WithPVCRole(ConfigVolumeRole).
			Build()

		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        galeraresources.GaleraConfigVolume,
				Labels:      labels,
				Annotations: vctpl.Annotations,
			},
			Spec: vctpl.PersistentVolumeClaimSpec,
		})
	}
	return pvcs
}

func maxscaleVolumeClaimTemplates(maxscale *mariadbv1alpha1.MaxScale) []corev1.PersistentVolumeClaim {
	vctpl := maxscale.Spec.Config.VolumeClaimTemplate
	pvcs := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        StorageVolume,
				Labels:      vctpl.Labels,
				Annotations: vctpl.Annotations,
			},
			Spec: vctpl.PersistentVolumeClaimSpec,
		},
	}
	return pvcs
}

func mariadbVolumes(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbOpt) []corev1.Volume {
	mariadbOpts := newMariadbOpts(opts...)
	configVolume := corev1.Volume{
		Name: ConfigVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	if mariadb.Spec.MyCnfConfigMapKeyRef != nil {
		configVolume = corev1.Volume{
			Name: ConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
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
	volumes := []corev1.Volume{
		configVolume,
	}
	if mariadb.Replication().Enabled && ptr.Deref(mariadb.Replication().ProbesEnabled, false) {
		volumes = append(volumes, corev1.Volume{
			Name: ProbesVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mariadb.ReplConfigMapKeyRef().Name,
					},
					DefaultMode: ptr.To(int32(0777)),
				},
			},
		})
	}
	if mariadb.IsGaleraEnabled() {
		volumes = append(volumes, corev1.Volume{
			Name: ServiceAccountVolume,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								Path: "token",
							},
						},
						{
							ConfigMap: &corev1.ConfigMapProjection{
								Items: []corev1.KeyToPath{
									{
										Key:  "ca.crt",
										Path: "ca.crt",
									},
								},
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "kube-root-ca.crt",
								},
							},
						},
						{
							DownwardAPI: &corev1.DownwardAPIProjection{
								Items: []corev1.DownwardAPIVolumeFile{
									{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
										Path: "namespace",
									},
								},
							},
						},
					},
				},
			},
		})
	}
	if mariadb.IsEphemeralStorageEnabled() {
		volumes = append(volumes, corev1.Volume{
			Name: StorageVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}
	if mariadb.Spec.Volumes != nil {
		volumes = append(volumes, mariadb.Spec.Volumes...)
	}
	if mariadbOpts.extraVolumes != nil {
		volumes = append(volumes, mariadbOpts.extraVolumes...)
	}
	return volumes
}

func maxscaleVolumes(maxscale *mariadbv1alpha1.MaxScale) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: ConfigVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: maxscale.ConfigSecretKeyRef().Name,
				},
			},
		},
	}
	if maxscale.Spec.Volumes != nil {
		volumes = append(volumes, maxscale.Spec.Volumes...)
	}
	return volumes
}

func mariadbHAAnnotations(mariadb *mariadbv1alpha1.MariaDB) map[string]string {
	var annotations map[string]string
	if mariadb.IsHAEnabled() {
		annotations = map[string]string{
			annotation.MariadbAnnotation: mariadb.Name,
		}
		if mariadb.Replication().Enabled {
			annotations[annotation.ReplicationAnnotation] = ""
		}
		if mariadb.IsGaleraEnabled() {
			annotations[annotation.GaleraAnnotation] = ""
		}
	}
	return annotations
}

func serviceAccount(svcAccount *string, defaultSvcAccount string) (serviceAccount string) {
	if svcAccount != nil {
		return *svcAccount
	}
	return defaultSvcAccount
}

func priorityClass(className *string) string {
	if className != nil {
		return *className
	}
	return ""
}
