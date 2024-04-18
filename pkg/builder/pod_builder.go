package builder

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	mdbptr "github.com/mariadb-operator/mariadb-operator/pkg/ptr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mariadbOpts struct {
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

func newMariadbOpts(userOpts ...mariadbOpt) *mariadbOpts {
	opts := &mariadbOpts{
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

type mariadbOpt func(opts *mariadbOpts)

func withMeta(meta *mariadbv1alpha1.Metadata) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.meta = meta
	}
}

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

func withResources(resources *corev1.ResourceRequirements) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.resources = resources
	}
}

func withAffinity(affinity *mariadbv1alpha1.AffinityConfig) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.affinity = affinity
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

func withMariadbSelectorLabels(includeSelectorLabels bool) mariadbOpt {
	return func(opts *mariadbOpts) {
		opts.includeSelectorLabels = includeSelectorLabels
	}
}

func (b *Builder) mariadbPodTemplate(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbOpt) *corev1.PodTemplateSpec {
	containers := b.mariadbContainers(mariadb, opts...)
	mariadbOpts := newMariadbOpts(opts...)

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
			InitContainers:               mariadbInitContainers(mariadb, opts...),
			Containers:                   containers,
			ImagePullSecrets:             mariadb.Spec.ImagePullSecrets,
			Volumes:                      mariadbVolumes(mariadb, opts...),
			SecurityContext:              mariadb.Spec.PodSecurityContext,
			Affinity:                     &affinity,
			NodeSelector:                 mariadb.Spec.NodeSelector,
			Tolerations:                  mariadb.Spec.Tolerations,
			PriorityClassName:            ptr.Deref(mariadb.Spec.PriorityClassName, ""),
			TopologySpreadConstraints:    mariadb.Spec.TopologySpreadConstraints,
		},
	}
}

func (b *Builder) maxscalePodTemplate(mxs *mariadbv1alpha1.MaxScale) *corev1.PodTemplateSpec {
	containers := b.maxscaleContainers(mxs)
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
	affinity := ptr.Deref(mxs.Spec.Affinity, mariadbv1alpha1.AffinityConfig{}).Affinity
	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           ptr.Deref(mxs.Spec.ServiceAccountName, mxs.Name),
			InitContainers:               maxscaleInitContainers(mxs),
			Containers:                   containers,
			ImagePullSecrets:             mxs.Spec.ImagePullSecrets,
			Volumes:                      maxscaleVolumes(mxs),
			SecurityContext:              mxs.Spec.PodSecurityContext,
			Affinity:                     &affinity,
			NodeSelector:                 mxs.Spec.NodeSelector,
			Tolerations:                  mxs.Spec.Tolerations,
			PriorityClassName:            ptr.Deref(mxs.Spec.PriorityClassName, ""),
			TopologySpreadConstraints:    mxs.Spec.TopologySpreadConstraints,
		},
	}
}
