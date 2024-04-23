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

func (b *Builder) BuildMaxScale(key types.NamespacedName, mdb *mariadbv1alpha1.MariaDB,
	mdbmxs *mariadbv1alpha1.MariaDBMaxScaleSpec) (*mariadbv1alpha1.MaxScale, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(mdb.Spec.InheritMetadata).
			Build()
	mxs := mariadbv1alpha1.MaxScale{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.MaxScaleSpec{
			MariaDBRef: &mariadbv1alpha1.MariaDBRef{
				ObjectReference: corev1.ObjectReference{
					Name:      mdb.Name,
					Namespace: mdb.Namespace,
				},
			},
			Image:                mdbmxs.Image,
			ImagePullPolicy:      mdbmxs.ImagePullPolicy,
			Services:             mdbmxs.Services,
			Monitor:              ptr.Deref(mdbmxs.Monitor, mariadbv1alpha1.MaxScaleMonitor{}),
			Admin:                ptr.Deref(mdbmxs.Admin, mariadbv1alpha1.MaxScaleAdmin{}),
			Config:               ptr.Deref(mdbmxs.Config, mariadbv1alpha1.MaxScaleConfig{}),
			Auth:                 ptr.Deref(mdbmxs.Auth, mariadbv1alpha1.MaxScaleAuth{}),
			Connection:           mdbmxs.Connection,
			Metrics:              mdbmxs.Metrics,
			Replicas:             ptr.Deref(mdbmxs.Replicas, 1),
			PodDisruptionBudget:  mdbmxs.PodDisruptionBudget,
			UpdateStrategy:       mdbmxs.UpdateStrategy,
			KubernetesService:    mdbmxs.KubernetesService,
			GuiKubernetesService: mdbmxs.GuiKubernetesService,
			RequeueInterval:      mdbmxs.RequeueInterval,
		},
	}
	if err := controllerutil.SetControllerReference(mdb, &mxs, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller to MaxScale %v", err)
	}
	return &mxs, nil
}
