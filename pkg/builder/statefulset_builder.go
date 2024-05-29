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

	RunVolume            = "run"
	MaxScaleRunMountPath = "/var/run/maxscale"

	LogVolume            = "log"
	MaxScaleLogMountPath = "/var/log/maxscale"

	CacheVolume            = "cache"
	MaxScaleCacheMountPath = "/var/cache/maxscale"

	InitVolume        = "init"
	InitConfigPath    = "/init"
	InitLibKey        = "lib.sh"
	InitEntrypointKey = "entrypoint.sh"

	ProbesVolume    = "probes"
	ProbesMountPath = "/etc/probes"

	ServiceAccountVolume    = "serviceaccount"
	ServiceAccountMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"

	mysqlUser               = int64(999)
	mysqlGroup              = int64(999)
	maxscaleUser            = int64(998)
	maxscaleGroup           = int64(996)
	maxscaleEnterpriseUser  = int64(999)
	maxscaleEnterpriseGroup = int64(999)
)

func (b *Builder) BuildMariadbStatefulSet(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName,
	podAnnotations map[string]string) (*appsv1.StatefulSet, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(mariadb.Spec.InheritMetadata).
			WithAnnotations(mariadbHAAnnotations(mariadb)).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDBSelectorLabels(mariadb).
			Build()

	updateStrategy, err := mariadbUpdateStrategy(mariadb)
	if err != nil {
		return nil, err
	}

	var mariadbPodOpts []mariadbPodOpt
	if podAnnotations != nil {
		mariadbPodOpts = append(mariadbPodOpts,
			withMeta(&mariadbv1alpha1.Metadata{
				Annotations: podAnnotations,
			}),
		)
	}
	podTemplate, err := b.mariadbPodTemplate(mariadb, mariadbPodOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building MariaDB Pod template: %v", err)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         mariadb.InternalServiceKey().Name,
			Replicas:            &mariadb.Spec.Replicas,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy:      *updateStrategy,
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
			WithMetadata(maxscale.Spec.InheritMetadata).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMaxScaleSelectorLabels(maxscale).
			Build()
	podTemplate, err := b.maxscalePodTemplate(maxscale)
	if err != nil {
		return nil, err
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

func mariadbUpdateStrategy(mdb *mariadbv1alpha1.MariaDB) (*appsv1.StatefulSetUpdateStrategy, error) {
	switch mdb.Spec.UpdateStrategy.Type {
	case mariadbv1alpha1.ReplicasFirstPrimaryLast:
		return &appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.OnDeleteStatefulSetStrategyType,
		}, nil
	case mariadbv1alpha1.RollingUpdateUpdateType:
		return &appsv1.StatefulSetUpdateStrategy{
			Type:          appsv1.RollingUpdateStatefulSetStrategyType,
			RollingUpdate: mdb.Spec.UpdateStrategy.RollingUpdate,
		}, nil
	case mariadbv1alpha1.OnDeleteUpdateType:
		return &appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.OnDeleteStatefulSetStrategyType,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported update strategy type: %v", mdb.Spec.UpdateStrategy.Type)
	}
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
	vctpl := mariadb.Spec.Storage.VolumeClaimTemplate

	if !mariadb.IsEphemeralStorageEnabled() && vctpl != nil {
		meta := ptr.Deref(vctpl.Metadata, mariadbv1alpha1.Metadata{})
		labels := labels.NewLabelsBuilder().
			WithLabels(meta.Labels).
			WithPVCRole(StorageVolumeRole).
			Build()

		pvcs = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:        StorageVolume,
					Labels:      labels,
					Annotations: meta.Annotations,
				},
				Spec: vctpl.PersistentVolumeClaimSpec,
			},
		}
	}

	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	reuseStorageVolume := ptr.Deref(galera.Config.ReuseStorageVolume, false)
	vctpl = galera.Config.VolumeClaimTemplate

	if mariadb.IsGaleraEnabled() && !reuseStorageVolume && vctpl != nil {
		meta := ptr.Deref(vctpl.Metadata, mariadbv1alpha1.Metadata{})
		labels := labels.NewLabelsBuilder().
			WithLabels(meta.Labels).
			WithPVCRole(ConfigVolumeRole).
			Build()

		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        galeraresources.GaleraConfigVolume,
				Labels:      labels,
				Annotations: meta.Annotations,
			},
			Spec: vctpl.PersistentVolumeClaimSpec,
		})
	}
	return pvcs
}

func maxscaleVolumeClaimTemplates(maxscale *mariadbv1alpha1.MaxScale) []corev1.PersistentVolumeClaim {
	vctpl := maxscale.Spec.Config.VolumeClaimTemplate
	meta := ptr.Deref(vctpl.Metadata, mariadbv1alpha1.Metadata{})
	pvcs := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        StorageVolume,
				Labels:      meta.Labels,
				Annotations: meta.Annotations,
			},
			Spec: vctpl.PersistentVolumeClaimSpec,
		},
	}
	return pvcs
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
