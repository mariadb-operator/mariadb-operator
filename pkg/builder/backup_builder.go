package builder

import (
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type BackupOpts struct {
	Metadata    []*mariadbv1alpha1.Metadata
	Key         types.NamespacedName
	MariaDBRef  mariadbv1alpha1.MariaDBRef
	Compression mariadbv1alpha1.CompressAlgorithm
	Storage     mariadbv1alpha1.BackupStorage
	Args        []string
	Tables      []string
	Resources   mariadbv1alpha1.ResourceRequirements
	Affinity    mariadbv1alpha1.AffinityConfig
	// MaxRetention metav1.Duration
	MaxRetention     time.Duration
	ImagePullSecrets []mariadbv1alpha1.LocalObjectReference
	// Template is an optional Backup whose Spec is used as the base for the new Backup. The fields managed
	// by the controller (Storage, MariaDBRef, Compression, Args, Tables, MaxRetention, ImagePullSecrets)
	// are overridden by the values in BackupOpts. The remaining fields (resources, pod template, etc.) are
	// preserved from the template, which lets callers customize the backup Pod via a templated Backup object.
	Template *mariadbv1alpha1.Backup
}

func (b *Builder) BuildBackup(opts BackupOpts, owner metav1.Object) (*mariadbv1alpha1.Backup, error) {
	objMetaBuilder :=
		metadata.NewMetadataBuilder(opts.Key)
	for _, meta := range opts.Metadata {
		objMetaBuilder = objMetaBuilder.WithMetadata(meta)
	}
	objMeta := objMetaBuilder.Build()

	var spec mariadbv1alpha1.BackupSpec
	if opts.Template != nil {
		spec = *opts.Template.Spec.DeepCopy()
	}
	spec.Storage = opts.Storage
	spec.MariaDBRef = opts.MariaDBRef
	spec.Compression = opts.Compression
	spec.Tables = opts.Tables
	spec.MaxRetention = metav1.Duration{Duration: opts.MaxRetention}
	spec.Args = opts.Args
	// The operator-managed Backup runs as a one-shot Job. A suspended Schedule on the template is what
	// makes the template object skip Job/CronJob reconciliation; clearing it here ensures the resulting
	// Backup is reconciled as a regular Job.
	spec.Schedule = nil
	if spec.Resources == nil {
		spec.Resources = &opts.Resources
	}
	if spec.Affinity == nil {
		spec.Affinity = &opts.Affinity
	}
	if len(opts.ImagePullSecrets) > 0 {
		spec.ImagePullSecrets = opts.ImagePullSecrets
	}

	backup := &mariadbv1alpha1.Backup{
		ObjectMeta: objMeta,
		Spec:       spec,
	}

	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, backup, b.scheme); err != nil {
			return nil, fmt.Errorf("error setting controller reference to Backup: %v", err)
		}
	}

	return backup, nil
}
