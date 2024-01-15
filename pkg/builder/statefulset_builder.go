package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	annotation "github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	StorageVolume            = "storage"
	MariadbStorageMountPath  = "/var/lib/mysql"
	MaxscaleStorageMountPath = "/var/lib/maxscale"

	ConfigVolume            = "config"
	MariadbConfigMountPath  = "/etc/mysql/conf.d"
	MaxscaleConfigMountPath = "/etc/config"

	ServiceAccountVolume    = "serviceaccount"
	ServiceAccountMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"

	MariadbContainerName = "mariadb"
	MariadbPortName      = "mariadb"

	InitContainerName  = "init"
	AgentContainerName = "agent"
)

func PVCKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	podName := statefulset.PodName(mariadb.ObjectMeta, 0)
	if mariadb.Replication().Enabled {
		podName = statefulset.PodName(mariadb.ObjectMeta, *mariadb.Replication().Primary.PodIndex)
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", StorageVolume, podName),
		Namespace: mariadb.Namespace,
	}
}

func (b *Builder) BuildMariadbStatefulSet(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) (*appsv1.StatefulSet, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			WithAnnotations(mariadbHAAnnotations(mariadb)).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDBSelectorLabels(mariadb).
			Build()
	podTemplate, err := b.mariadbPodTemplate(mariadb, selectorLabels)
	if err != nil {
		return nil, fmt.Errorf("error building pod template: %v", err)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         mariadb.InternalServiceKey().Name,
			Replicas:            &mariadb.Spec.Replicas,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy:      statefulSetUpdateStrategy(mariadb.Spec.UpdateStrategy),
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
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMaxScaleSelectorLabels(maxscale).
			Build()
	podTemplate, err := b.maxscalePodTemplate(maxscale, selectorLabels)
	if err != nil {
		return nil, fmt.Errorf("error building pod template: %v", err)
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

func (b *Builder) mariadbPodTemplate(mariadb *mariadbv1alpha1.MariaDB, labels map[string]string) (*corev1.PodTemplateSpec, error) {
	containers, err := b.mariadbContainers(mariadb)
	if err != nil {
		return nil, fmt.Errorf("error building MariaDB containers: %v", err)
	}
	objMeta :=
		metadata.NewMetadataBuilder(client.ObjectKeyFromObject(mariadb)).
			WithMariaDB(mariadb).
			WithLabels(labels).
			WithAnnotations(mariadb.Spec.PodAnnotations).
			WithAnnotations(mariadbHAAnnotations(mariadb)).
			Build()
	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           mariadbServiceAccountName(mariadb),
			InitContainers:               mariadbInitContainers(mariadb),
			Containers:                   containers,
			ImagePullSecrets:             mariadb.Spec.ImagePullSecrets,
			Volumes:                      mariadbVolumes(mariadb),
			SecurityContext:              mariadb.Spec.PodSecurityContext,
			Affinity:                     mariadb.Spec.Affinity,
			NodeSelector:                 mariadb.Spec.NodeSelector,
			Tolerations:                  mariadb.Spec.Tolerations,
			PriorityClassName:            buildPriorityClass(mariadb),
			TopologySpreadConstraints:    mariadb.Spec.TopologySpreadConstraints,
		},
	}, nil
}

func (b *Builder) maxscalePodTemplate(maxscale *mariadbv1alpha1.MaxScale, labels map[string]string) (*corev1.PodTemplateSpec, error) {
	containers, err := b.maxscaleContainers(maxscale)
	if err != nil {
		return nil, fmt.Errorf("error building MaxScale containers: %v", err)
	}
	objMeta :=
		metadata.NewMetadataBuilder(client.ObjectKeyFromObject(maxscale)).
			WithLabels(labels).
			Build()
	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           "default",
			Containers:                   containers,
			ImagePullSecrets:             maxscale.Spec.ImagePullSecrets,
			Volumes:                      maxscaleVolumes(maxscale),
			SecurityContext:              maxscale.Spec.PodSecurityContext,
			Affinity:                     maxscale.Spec.Affinity,
			NodeSelector:                 maxscale.Spec.NodeSelector,
			Tolerations:                  maxscale.Spec.Tolerations,
		},
	}, nil
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

	if !mariadb.IsEphemeralStorageEnabled() {
		vctpl := mariadb.Spec.VolumeClaimTemplate
		pvcs = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:        StorageVolume,
					Labels:      vctpl.Labels,
					Annotations: vctpl.Annotations,
				},
				Spec: vctpl.PersistentVolumeClaimSpec,
			},
		}
	}

	if mariadb.Galera().Enabled {
		vctpl := *mariadb.Galera().VolumeClaimTemplate
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        galeraresources.GaleraConfigVolume,
				Labels:      vctpl.Labels,
				Annotations: vctpl.Annotations,
			},
			Spec: vctpl.PersistentVolumeClaimSpec,
		})
	}
	return pvcs
}

func maxscaleVolumeClaimTemplates(maxscale *mariadbv1alpha1.MaxScale) []corev1.PersistentVolumeClaim {
	vctpl := maxscale.Spec.Config.VolumeClaimTemplate
	pvcs := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        StorageVolume,
				Labels:      vctpl.Labels,
				Annotations: vctpl.Annotations,
			},
			Spec: vctpl.PersistentVolumeClaimSpec,
		},
	}
	return pvcs
}

func mariadbVolumes(mariadb *mariadbv1alpha1.MariaDB) []corev1.Volume {
	configVolume := corev1.Volume{
		Name: ConfigVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	if mariadb.Spec.MyCnfConfigMapKeyRef != nil {
		configVolume = corev1.Volume{
			Name: ConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mariadb.Spec.MyCnfConfigMapKeyRef.Name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  mariadb.Spec.MyCnfConfigMapKeyRef.Key,
							Path: "my.cnf",
						},
					},
				},
			},
		}
	}
	volumes := []corev1.Volume{
		configVolume,
	}
	if mariadb.Galera().Enabled {
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
	return volumes
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
	}
	if maxscale.Spec.Volumes != nil {
		volumes = append(volumes, maxscale.Spec.Volumes...)
	}
	return volumes
}

func mariadbServiceAccountName(mariadb *mariadbv1alpha1.MariaDB) (serviceAccount string) {
	if mariadb.Spec.ServiceAccountName != nil {
		return *mariadb.Spec.ServiceAccountName
	}
	return mariadb.Name
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
		if mariadb.Galera().Enabled {
			annotations[annotation.GaleraAnnotation] = ""
		}
	}
	return annotations
}

func buildPriorityClass(mariadb *mariadbv1alpha1.MariaDB) string {
	if mariadb.Spec.PodTemplate.PriorityClassName != nil {
		return *mariadb.Spec.PodTemplate.PriorityClassName
	}
	return ""
}
