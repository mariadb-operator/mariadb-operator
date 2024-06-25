package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	mdbptr "github.com/mariadb-operator/mariadb-operator/pkg/ptr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mariadbPodOpts struct {
	meta                  *mariadbv1alpha1.Metadata
	command               []string
	args                  []string
	restartPolicy         *corev1.RestartPolicy
	resources             *corev1.ResourceRequirements
	affinity              *mariadbv1alpha1.AffinityConfig
	extraVolumes          []corev1.Volume
	extraVolumeMounts     []corev1.VolumeMount
	includeGalera         bool
	includePorts          bool
	includeProbes         bool
	includeSelectorLabels bool
}

func newMariadbPodOpts(userOpts ...mariadbPodOpt) *mariadbPodOpts {
	opts := &mariadbPodOpts{
		includeGalera:         true,
		includePorts:          true,
		includeProbes:         true,
		includeSelectorLabels: true,
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

func withAffinity(affinity *mariadbv1alpha1.AffinityConfig) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.affinity = affinity
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

func withGalera(includeGalera bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeGalera = includeGalera
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

func withMariadbSelectorLabels(includeSelectorLabels bool) mariadbPodOpt {
	return func(opts *mariadbPodOpts) {
		opts.includeSelectorLabels = includeSelectorLabels
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
	if mariadbOpts.includeSelectorLabels {
		selectorLabels :=
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mariadb).
				Build()
		objMetaBuilder = objMetaBuilder.WithLabels(selectorLabels)
	}
	objMeta := objMetaBuilder.
		WithAnnotations(mariadbHAAnnotations(mariadb)).
		Build()

	initContainers, err := b.mariadbInitContainers(mariadb, opts...)
	if err != nil {
		return nil, err
	}

	securityContext, err := b.buildPodSecurityContextWithUserGroup(mariadb.Spec.PodSecurityContext, mysqlUser, mysqlGroup)
	if err != nil {
		return nil, err
	}

	affinity := mdbptr.Deref(
		[]*mariadbv1alpha1.AffinityConfig{
			mariadbOpts.affinity,
			mariadb.Spec.Affinity,
		},
		mariadbv1alpha1.AffinityConfig{},
	).Affinity

	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           ptr.Deref(mariadb.Spec.ServiceAccountName, mariadb.Name),
			RestartPolicy:                ptr.Deref(mariadbOpts.restartPolicy, corev1.RestartPolicyAlways),
			InitContainers:               initContainers,
			Containers:                   containers,
			ImagePullSecrets:             mariadb.Spec.ImagePullSecrets,
			Volumes:                      mariadbVolumes(mariadb, opts...),
			SecurityContext:              securityContext,
			Affinity:                     &affinity,
			NodeSelector:                 mariadb.Spec.NodeSelector,
			Tolerations:                  mariadb.Spec.Tolerations,
			PriorityClassName:            ptr.Deref(mariadb.Spec.PriorityClassName, ""),
			TopologySpreadConstraints:    mariadb.Spec.TopologySpreadConstraints,
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

	initContainers, err := b.maxscaleInitContainers(mxs)
	if err != nil {
		return nil, err
	}
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
			InitContainers:               initContainers,
			Containers:                   containers,
			ImagePullSecrets:             mxs.Spec.ImagePullSecrets,
			Volumes:                      maxscaleVolumes(mxs),
			SecurityContext:              securityContext,
			Affinity:                     &affinity,
			NodeSelector:                 mxs.Spec.NodeSelector,
			Tolerations:                  mxs.Spec.Tolerations,
			PriorityClassName:            ptr.Deref(mxs.Spec.PriorityClassName, ""),
			TopologySpreadConstraints:    mxs.Spec.TopologySpreadConstraints,
		},
	}, nil
}

func (b *Builder) maxscalePodSecurityContext(mxs *mariadbv1alpha1.MaxScale) (*corev1.PodSecurityContext, error) {
	if b.discovery.IsEnterprise() {
		return b.buildPodSecurityContextWithUserGroup(mxs.Spec.PodSecurityContext, maxscaleEnterpriseUser, maxscaleEnterpriseGroup)
	}
	return b.buildPodSecurityContextWithUserGroup(mxs.Spec.PodSecurityContext, maxscaleUser, maxscaleGroup)
}

func mariadbVolumes(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) []corev1.Volume {
	mariadbOpts := newMariadbPodOpts(opts...)
	volumes := []corev1.Volume{
		mariadbConfigVolume(mariadb),
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
	return corev1.Volume{
		Name: ConfigVolume,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: projections,
			},
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
	if maxscale.Spec.Volumes != nil {
		volumes = append(volumes, maxscale.Spec.Volumes...)
	}
	return volumes
}
