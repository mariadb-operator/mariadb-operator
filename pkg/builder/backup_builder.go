package builder

import (
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/metadata"
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
	Resources   mariadbv1alpha1.ResourceRequirements
	Affinity    mariadbv1alpha1.AffinityConfig
	// MaxRetention metav1.Duration
	MaxRetention     time.Duration
	ImagePullSecrets []mariadbv1alpha1.LocalObjectReference
}

func (b *Builder) BuildBackup(opts BackupOpts, owner metav1.Object) (*mariadbv1alpha1.Backup, error) {
	objMetaBuilder :=
		metadata.NewMetadataBuilder(opts.Key)
	for _, meta := range opts.Metadata {
		objMetaBuilder = objMetaBuilder.WithMetadata(meta)
	}
	objMeta := objMetaBuilder.Build()

	backup := &mariadbv1alpha1.Backup{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.BackupSpec{
			Storage:     opts.Storage,
			MariaDBRef:  opts.MariaDBRef,
			Compression: opts.Compression,

			MaxRetention: metav1.Duration{
				Duration: opts.MaxRetention,
			},
			JobContainerTemplate: mariadbv1alpha1.JobContainerTemplate{
				Args:      opts.Args,
				Resources: &opts.Resources,
			},
			JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
				Affinity: &opts.Affinity,
			},
		},
	}

	if len(opts.ImagePullSecrets) > 0 {
		backup.Spec.ImagePullSecrets = opts.ImagePullSecrets
	}

	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, backup, b.scheme); err != nil {
			return nil, fmt.Errorf("error setting controller reference to Backup: %v", err)
		}
	}

	return backup, nil
}
