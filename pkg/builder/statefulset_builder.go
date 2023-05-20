package builder

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/annotation"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	stsStorageVolume            = "storage"
	stsStorageMountPath         = "/var/lib/mysql"
	stsConfigVolume             = "config"
	stsConfigMountPath          = "/etc/mysql/conf.d"
	stsGaleraTplConfigVolume    = "galera-tpl"
	stsGaleraTplConfigMountPath = "/galera-tpl"
	stsGaleraConfigVolume       = "galera"
	stsGaleraConfigMountPath    = "/etc/mysql/mariadb.conf.d"

	mariaDbContainerName = "mariadb"
	mariaDbPortName      = "mariadb"

	metricsContainerName = "metrics"
	metricsPortName      = "metrics"
	metricsPort          = 9104
)

func PVCKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	podName := statefulset.PodName(mariadb.ObjectMeta, 0)
	if mariadb.Spec.Replication != nil {
		podName = statefulset.PodName(mariadb.ObjectMeta, mariadb.Spec.Replication.Primary.PodIndex)
	}
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", stsStorageVolume, podName),
		Namespace: mariadb.Namespace,
	}
}

func StatefulSetPort(sts *appsv1.StatefulSet) (*corev1.ContainerPort, error) {
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == mariaDbContainerName {
			for _, p := range c.Ports {
				if p.Name == mariaDbPortName {
					return &p, nil
				}
			}
		}
	}
	return nil, errors.New("StatefulSet port not found")
}

func (b *Builder) BuildStatefulSet(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName,
	dsn *corev1.SecretKeySelector) (*appsv1.StatefulSet, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDBSelectorLabels(mariadb).
			Build()
	podTemplate, err := buildStsPodTemplate(mariadb, dsn, selectorLabels)
	if err != nil {
		return nil, fmt.Errorf("error building pod template: %v", err)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         mariadb.Name,
			Replicas:            &mariadb.Spec.Replicas,
			PodManagementPolicy: buildStsPodManagementPolicy(mariadb),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template:             *podTemplate,
			VolumeClaimTemplates: buildStsVolumeClaimTemplates(mariadb),
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, sts, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to StatefulSet: %v", err)
	}
	return sts, nil
}

func buildStsPodManagementPolicy(mariadb *mariadbv1alpha1.MariaDB) appsv1.PodManagementPolicyType {
	if mariadb.Spec.Replication != nil || mariadb.Spec.Galera != nil {
		return appsv1.ParallelPodManagement
	}
	return appsv1.OrderedReadyPodManagement
}

func buildStsVolumeClaimTemplates(mariadb *mariadbv1alpha1.MariaDB) []corev1.PersistentVolumeClaim {
	pvcs := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: stsStorageVolume,
			},
			Spec: mariadb.Spec.VolumeClaimTemplate,
		},
	}
	if mariadb.Spec.Galera != nil {
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: stsGaleraConfigVolume,
			},
			Spec: mariadb.Spec.Galera.ConfigVolumeClaimTemplate,
		})
	}
	return pvcs
}

func buildStsPodTemplate(mariadb *mariadbv1alpha1.MariaDB, dsn *corev1.SecretKeySelector,
	labels map[string]string) (*corev1.PodTemplateSpec, error) {
	containers, err := buildStsContainers(mariadb, dsn)
	if err != nil {
		return nil, fmt.Errorf("error building MariaDB containers: %v", err)
	}

	var podAnnotations map[string]string
	if mariadb.Spec.Replication != nil || mariadb.Spec.Galera != nil {
		podAnnotations = map[string]string{
			annotation.PodReplicationAnnotation: "true",
			annotation.PodMariadbAnnotation:     mariadb.Name,
		}
	}
	objMeta :=
		metadata.NewMetadataBuilder(client.ObjectKeyFromObject(mariadb)).
			WithMariaDB(mariadb).
			WithLabels(labels).
			WithAnnotations(podAnnotations).
			Build()

	return &corev1.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: corev1.PodSpec{
			InitContainers:  buildStsInitContainers(mariadb),
			Containers:      containers,
			Volumes:         buildStsVolumes(mariadb),
			SecurityContext: mariadb.Spec.PodSecurityContext,
			Affinity:        mariadb.Spec.Affinity,
			NodeSelector:    mariadb.Spec.NodeSelector,
			Tolerations:     mariadb.Spec.Tolerations,
		},
	}, nil
}

func buildStsVolumes(mariadb *mariadbv1alpha1.MariaDB) []corev1.Volume {
	configVolume := corev1.Volume{
		Name: stsConfigVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	if mariadb.Spec.MyCnfConfigMapKeyRef != nil {
		configVolume = corev1.Volume{
			Name: stsConfigVolume,
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
	if mariadb.Spec.Galera != nil {
		volumes = append(volumes, corev1.Volume{
			Name: stsGaleraTplConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: galeraresources.ConfigMapKey(mariadb).Name,
					},
				},
			},
		})
	}
	if mariadb.Spec.Volumes != nil {
		volumes = append(volumes, mariadb.Spec.Volumes...)
	}
	return volumes
}
