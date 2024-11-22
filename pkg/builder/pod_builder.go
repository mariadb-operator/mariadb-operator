package builder

import (
	"fmt"
	"reflect"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	builderpki "github.com/mariadb-operator/mariadb-operator/pkg/builder/pki"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	kadapter "github.com/mariadb-operator/mariadb-operator/pkg/kubernetes/adapter"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mariadbPodOpts struct {
	meta                         *mariadbv1alpha1.Metadata
	command                      []string
	args                         []string
	restartPolicy                *corev1.RestartPolicy
	resources                    *corev1.ResourceRequirements
	affinity                     *corev1.Affinity
	nodeSelector                 map[string]string
	extraVolumes                 []corev1.Volume
	extraVolumeMounts            []corev1.VolumeMount
	includeMariadbResources      bool
	includeMariadbSelectorLabels bool
	includeGaleraContainers      bool
	includeGaleraConfig          bool
	includeServiceAccount        bool
	includePorts                 bool
	includeProbes                bool
	includeHAAnnotations         bool
	includeAffinity              bool
}

func newMariadbPodOpts(userOpts ...mariadbPodOpt) *mariadbPodOpts {
	opts := &mariadbPodOpts{
		includeMariadbResources:      true,
		includeMariadbSelectorLabels: true,
		includeGaleraContainers:      true,
		includeGaleraConfig:          true,
		includeServiceAccount:        true,
		includePorts:                 true,
		includeProbes:                true,
		includeHAAnnotations:         true,
		includeAffinity:              true,
	}
	for _, setOpt := range userOpts {
		setOpt(opts)
	}
	return opts
}

type mariadbPodOpt func(opts *mariadbPodOpts)

func withMeta(meta *mariadbv1alpha1.Metadata) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.meta = meta
	}
}

func withCommand(command []string) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.command = command
	}
}

func withArgs(args []string) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.args = args
	}
}

func withRestartPolicy(restartPolicy corev1.RestartPolicy) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.restartPolicy = &restartPolicy
	}
}

func withResources(resources *corev1.ResourceRequirements) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.resources = resources
	}
}

func withAffinity(affinity *corev1.Affinity) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.affinity = affinity
	}
}

func withAffinityEnabled(includeAffinity bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeAffinity = includeAffinity
	}
}

func withNodeSelector(nodeSelector map[string]string) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.nodeSelector = nodeSelector
	}
}

func withExtraVolumes(volumes []corev1.Volume) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.extraVolumes = volumes
	}
}

func withExtraVolumeMounts(volumeMounts []corev1.VolumeMount) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.extraVolumeMounts = volumeMounts
	}
}

func withMariadbResources(includeMariadbResources bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeMariadbResources = includeMariadbResources
	}
}

func withMariadbSelectorLabels(includeMariadbSelectorLabels bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeMariadbSelectorLabels = includeMariadbSelectorLabels
	}
}

func withGaleraContainers(includeGaleraContainers bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeGaleraContainers = includeGaleraContainers
	}
}

func withGaleraConfig(includeGaleraConfig bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeGaleraConfig = includeGaleraConfig
	}
}

func withServiceAccount(includeServiceAccount bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeServiceAccount = includeServiceAccount
	}
}

func withPorts(includePorts bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includePorts = includePorts
	}
}

func withProbes(includeProbes bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeProbes = includeProbes
	}
}

func withHAAnnotations(includeHAAnnotations bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeHAAnnotations = includeHAAnnotations
	}
}

func (b *Builder) mariadbPodTemplate(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) (*corev1.PodTemplateSpec, error) {
	containers, err := b.mariadbContainers(mariadb, opts...)
	if err != nil {
		return nil, fmt.Errorf("error building MariaDB containers: %v", err)
	}
	mariadbOpts := newMariadbPodOpts(opts...)

	objMetaBuilder :=
		metadata.NewMetadataBuilder(client.ObjectKeyFromObject(mariadb)).
			WithMetadata(mariadb.Spec.InheritMetadata).
			WithMetadata(mariadb.Spec.PodMetadata).
			WithMetadata(mariadbOpts.meta)
	if mariadbOpts.includeMariadbSelectorLabels {
		selectorLabels :=
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mariadb).
				Build()
		objMetaBuilder = objMetaBuilder.WithLabels(selectorLabels)
	}
	if mariadbOpts.includeHAAnnotations {
		objMetaBuilder = objMetaBuilder.WithAnnotations(mariadbHAAnnotations(mariadb))
	}
	objMeta := objMetaBuilder.Build()

	initContainers, err := b.mariadbInitContainers(mariadb, opts...)
	if err != nil {
		return nil, err
	}

	securityContext, err := b.buildPodSecurityContextWithUserGroup(mariadb.Spec.PodSecurityContext, mysqlUser, mysqlGroup)
	if err != nil {
		return nil, err
	}

	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           mariadbServiceAccount(mariadb, opts...),
			RestartPolicy:                ptr.Deref(mariadbOpts.restartPolicy, corev1.RestartPolicyAlways),
			InitContainers:               initContainers,
			Containers:                   containers,
			ImagePullSecrets:             kadapter.ToKubernetesSlice(mariadb.Spec.ImagePullSecrets),
			Volumes:                      mariadbVolumes(mariadb, opts...),
			SecurityContext:              securityContext,
			Affinity:                     mariadbAffinity(mariadb, opts...),
			NodeSelector:                 mariadbNodeSelector(mariadb, opts...),
			Tolerations:                  mariadb.Spec.Tolerations,
			PriorityClassName:            ptr.Deref(mariadb.Spec.PriorityClassName, ""),
			TopologySpreadConstraints:    mariadbTopologySpreadConstraints(mariadb, opts...),
		},
	}, nil
}

func (b *Builder) maxscalePodTemplate(mxs *mariadbv1alpha1.MaxScale) (*corev1.PodTemplateSpec, error) {
	containers, err := b.maxscaleContainers(mxs)
	if err != nil {
		return nil, err
	}

	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMaxScaleSelectorLabels(mxs).
			Build()
	objMeta :=
		metadata.NewMetadataBuilder(client.ObjectKeyFromObject(mxs)).
			WithMetadata(mxs.Spec.InheritMetadata).
			WithMetadata(mxs.Spec.PodMetadata).
			WithLabels(selectorLabels).
			Build()

	securityContext, err := b.maxscalePodSecurityContext(mxs)
	if err != nil {
		return nil, err
	}
	affinity := ptr.Deref(mxs.Spec.Affinity, mariadbv1alpha1.AffinityConfig{}).Affinity

	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           ptr.Deref(mxs.Spec.ServiceAccountName, mxs.Name),
			Containers:                   containers,
			ImagePullSecrets:             kadapter.ToKubernetesSlice(mxs.Spec.ImagePullSecrets),
			Volumes:                      maxscaleVolumes(mxs),
			SecurityContext:              securityContext,
			Affinity:                     ptr.To(affinity.ToKubernetesType()),
			NodeSelector:                 mxs.Spec.NodeSelector,
			Tolerations:                  mxs.Spec.Tolerations,
			PriorityClassName:            ptr.Deref(mxs.Spec.PriorityClassName, ""),
			TopologySpreadConstraints:    kadapter.ToKubernetesSlice(mxs.Spec.TopologySpreadConstraints),
		},
	}, nil
}

func (b *Builder) maxscalePodSecurityContext(mxs *mariadbv1alpha1.MaxScale) (*corev1.PodSecurityContext, error) {
	if b.discovery.IsEnterprise() {
		return b.buildPodSecurityContextWithUserGroup(mxs.Spec.PodSecurityContext, maxscaleEnterpriseUser, maxscaleEnterpriseGroup)
	}
	return b.buildPodSecurityContextWithUserGroup(mxs.Spec.PodSecurityContext, maxscaleUser, maxscaleGroup)
}

func mariadbAffinity(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) *corev1.Affinity {
	mariadbOpts := newMariadbPodOpts(opts...)

	if !mariadbOpts.includeAffinity {
		return nil
	}
	if mariadbOpts.affinity != nil {
		return mariadbOpts.affinity
	}
	if mariadb.Spec.Affinity != nil {
		return ptr.To(mariadb.Spec.Affinity.ToKubernetesType())
	}
	return nil
}

func mariadbNodeSelector(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) map[string]string {
	mariadbOpts := newMariadbPodOpts(opts...)

	if mariadbOpts.nodeSelector != nil {
		return mariadbOpts.nodeSelector
	}
	return mariadb.Spec.NodeSelector
}

func mariadbTopologySpreadConstraints(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) []corev1.TopologySpreadConstraint {
	mariadbOpts := newMariadbPodOpts(opts...)

	if !mariadbOpts.includeAffinity {
		return nil
	}
	return kadapter.ToKubernetesSlice(mariadb.Spec.TopologySpreadConstraints)
}

func mariadbServiceAccount(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) string {
	mariadbOpts := newMariadbPodOpts(opts...)
	if !mariadbOpts.includeServiceAccount {
		return ""
	}
	return ptr.Deref(mariadb.Spec.ServiceAccountName, mariadb.Name)
}

func mariadbVolumes(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) []corev1.Volume {
	mariadbOpts := newMariadbPodOpts(opts...)
	volumes := []corev1.Volume{
		mariadbConfigVolume(mariadb),
	}
	if mariadb.IsTLSEnabled() {
		tlsVolumes, _ := mariadbTLSVolumes(mariadb)
		volumes = append(volumes, tlsVolumes...)
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

	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})

	if galera.Enabled {
		basicAuth := ptr.Deref(galera.Agent.BasicAuth, mariadbv1alpha1.BasicAuth{})

		if mariadbOpts.includeGaleraConfig && basicAuth.Enabled && !reflect.ValueOf(basicAuth.PasswordSecretKeyRef).IsZero() {
			volumes = append(volumes, corev1.Volume{
				Name: galeraresources.AgentAuthVolume,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: basicAuth.PasswordSecretKeyRef.Name,
					},
				},
			})
		}

		if mariadbOpts.includeServiceAccount {
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
		volumes = append(volumes, kadapter.ToKubernetesSlice(mariadb.Spec.Volumes)...)
	}
	if mariadbOpts.extraVolumes != nil {
		volumes = append(volumes, mariadbOpts.extraVolumes...)
	}
	return volumes
}

func mariadbConfigVolume(mariadb *mariadbv1alpha1.MariaDB) corev1.Volume {
	defaultConfigMapKeyRef := mariadb.DefaultConfigMapKeyRef()
	projections := []corev1.VolumeProjection{
		{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: defaultConfigMapKeyRef.Name,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  defaultConfigMapKeyRef.Key,
						Path: defaultConfigMapKeyRef.Key,
					},
				},
			},
		},
	}
	if mariadb.Spec.MyCnfConfigMapKeyRef != nil {
		projections = append(projections, corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mariadb.Spec.MyCnfConfigMapKeyRef.Name,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  mariadb.Spec.MyCnfConfigMapKeyRef.Key,
						Path: mariadb.Spec.MyCnfConfigMapKeyRef.Key,
					},
				},
			},
		})
	}
	if mariadb.IsTLSEnabled() {
		configMapKeyRef := mariadb.TLSConfigMapKeyRef()
		projections = append(projections, corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapKeyRef.Name,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  configMapKeyRef.Key,
						Path: configMapKeyRef.Key,
					},
				},
			},
		})
	}
	return corev1.Volume{
		Name: ConfigVolume,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: projections,
			},
		},
	}
}

func mariadbTLSVolumes(mariadb *mariadbv1alpha1.MariaDB) ([]corev1.Volume, []corev1.VolumeMount) {
	if !mariadb.IsTLSEnabled() {
		return nil, nil
	}
	return []corev1.Volume{
			{
				Name: builderpki.PKIVolume,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: mariadb.TLSCABundleSecretKeyRef().Name,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  pki.CACertKey,
											Path: pki.CACertKey,
										},
									},
								},
							},
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: mariadb.TLSClientCertSecretKey().Name,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  pki.TLSCertKey,
											Path: "client.crt",
										},
										{
											Key:  pki.TLSKeyKey,
											Path: "client.key",
										},
									},
								},
							},
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: mariadb.TLSServerCertSecretKey().Name,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  pki.TLSCertKey,
											Path: "server.crt",
										},
										{
											Key:  pki.TLSKeyKey,
											Path: "server.key",
										},
									},
								},
							},
						},
					},
				},
			},
		}, []corev1.VolumeMount{
			{
				Name:      builderpki.PKIVolume,
				MountPath: builderpki.MariadbPKIMountPath,
			},
		}
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
		{
			Name: RunVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: LogVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: CacheVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	return volumes
}
