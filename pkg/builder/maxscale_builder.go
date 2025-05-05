package builder

import (
	"fmt"
	"github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildMaxScale(key types.NamespacedName, mdb *mariadbv1alpha1.MariaDB,
	mdbmxs *mariadbv1alpha1.MariaDBMaxScaleSpec) (*v1alpha1.MaxScale, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(mdb.Spec.InheritMetadata).
			Build()
	mxs := v1alpha1.MaxScale{
		ObjectMeta: objMeta,
		Spec: v1alpha1.MaxScaleSpec{
			MariaDBRef: &v1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name:      mdb.Name,
					Namespace: mdb.Namespace,
				},
			},
			Image:                mdbmxs.Image,
			ImagePullPolicy:      mdbmxs.ImagePullPolicy,
			Services:             mdbmxs.Services,
			Monitor:              ptr.Deref(mdbmxs.Monitor, v1alpha1.MaxScaleMonitor{}),
			Admin:                ptr.Deref(mdbmxs.Admin, v1alpha1.MaxScaleAdmin{}),
			Config:               ptr.Deref(mdbmxs.Config, v1alpha1.MaxScaleConfig{}),
			Auth:                 ptr.Deref(mdbmxs.Auth, v1alpha1.MaxScaleAuth{}),
			Connection:           mdbmxs.Connection,
			Metrics:              mdbmxs.Metrics,
			TLS:                  mdbmxs.TLS,
			Replicas:             ptr.Deref(mdbmxs.Replicas, 1),
			PodDisruptionBudget:  mdbmxs.PodDisruptionBudget,
			UpdateStrategy:       mdbmxs.UpdateStrategy,
			KubernetesService:    mdbmxs.KubernetesService,
			GuiKubernetesService: mdbmxs.GuiKubernetesService,
			RequeueInterval:      mdbmxs.RequeueInterval,
		},
	}
	// TLS should be enforced in MariaDB to be enabled in MaxScale by default
	if mxs.Spec.TLS == nil && mdb != nil && mdb.IsTLSRequired() {
		mxs.Spec.TLS = &v1alpha1.MaxScaleTLS{
			Enabled: true,
		}
	}
	if err := controllerutil.SetControllerReference(mdb, &mxs, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller to MaxScale %v", err)
	}
	return &mxs, nil
}
