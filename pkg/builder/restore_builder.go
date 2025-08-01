package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/metadata"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildRestore(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) (*mariadbv1alpha1.Restore, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(mariadb.Spec.InheritMetadata).
			Build()
	bootstrapFrom := ptr.Deref(mariadb.Spec.BootstrapFrom, mariadbv1alpha1.BootstrapFrom{})
	restoreJob := ptr.Deref(bootstrapFrom.RestoreJob, mariadbv1alpha1.Job{})

	podTpl := mariadbv1alpha1.JobPodTemplate{}
	podTpl.FromPodTemplate(mariadb.Spec.PodTemplate.DeepCopy())
	podTpl.Affinity = restoreJob.Affinity
	podTpl.PodMetadata = mariadbv1alpha1.MergeMetadata(
		mariadb.Spec.InheritMetadata,
		restoreJob.Metadata,
	)

	// Allow the restoreJob to overwrite tolerations and node selector to ensure we allow the restore job to run
	// differently than the mariadb pods.
	if restoreJob.NodeSelector != nil {
		podTpl.NodeSelector = restoreJob.NodeSelector
	}
	if restoreJob.Tolerations != nil {
		podTpl.Tolerations = restoreJob.Tolerations
	}

	containerTpl := mariadbv1alpha1.JobContainerTemplate{}
	containerTpl.FromContainerTemplate(mariadb.Spec.ContainerTemplate.DeepCopy())
	containerTpl.Resources = restoreJob.Resources
	containerTpl.Args = restoreJob.Args

	restoreSource, err := bootstrapFrom.RestoreSource()
	if err != nil {
		return nil, fmt.Errorf("error getting restore source: %v", err)
	}

	restore := &mariadbv1alpha1.Restore{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.RestoreSpec{
			JobContainerTemplate: containerTpl,
			JobPodTemplate:       podTpl,
			RestoreSource:        *restoreSource,
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name: mariadb.Name,
				},
				WaitForIt: true,
			},
		},
	}
	if restoreJob.Metadata != nil {
		restore.Spec.InheritMetadata = restoreJob.Metadata
	}

	if err := controllerutil.SetControllerReference(mariadb, restore, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to restore Job: %v", err)
	}
	return restore, nil
}
