package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
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

	containerTpl := mariadbv1alpha1.JobContainerTemplate{}
	containerTpl.FromContainerTemplate(mariadb.Spec.ContainerTemplate.DeepCopy())
	containerTpl.Resources = restoreJob.Resources
	containerTpl.Args = restoreJob.Args

	restore := &mariadbv1alpha1.Restore{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.RestoreSpec{
			JobContainerTemplate: containerTpl,
			JobPodTemplate:       podTpl,
			RestoreSource:        bootstrapFrom.RestoreSource,
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: corev1.ObjectReference{
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
