package builders

import (
	"fmt"
	"strconv"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	stsStorageVolume    = "storage"
	stsStorageMountPath = "/var/lib/mysql"
)

func GetPVCKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s-0", stsStorageVolume, mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func BuildStatefulSet(mariadb *databasev1alpha1.MariaDB, key types.NamespacedName) (*appsv1.StatefulSet, error) {
	containers, err := buildStsContainers(mariadb)
	if err != nil {
		return nil, err
	}
	labels :=
		NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			WithComponent(componentDatabase).
			Build()
	pvcMeta := metav1.ObjectMeta{
		Name:      stsStorageVolume,
		Namespace: mariadb.Namespace,
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: mariadb.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mariadb.Name,
					Namespace: mariadb.Namespace,
					Labels:    labels,
				},
				Spec: v1.PodSpec{
					Containers: containers,
				},
			},
			VolumeClaimTemplates: []v1.PersistentVolumeClaim{
				*BuildPVC(pvcMeta, &mariadb.Spec.Storage),
			},
		},
	}, nil
}

func BuildPVC(meta metav1.ObjectMeta, storage *databasev1alpha1.Storage) *v1.PersistentVolumeClaim {
	accessModes := storage.AccessModes
	if accessModes == nil {
		accessModes = []v1.PersistentVolumeAccessMode{
			v1.ReadWriteOnce,
		}
	}

	return &v1.PersistentVolumeClaim{
		ObjectMeta: meta,
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      accessModes,
			StorageClassName: &storage.ClassName,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: storage.Size,
				},
			},
		},
	}
}

func buildStsContainers(mariadb *databasev1alpha1.MariaDB) ([]v1.Container, error) {
	container := v1.Container{
		Name:            mariadb.Name,
		Image:           mariadb.Spec.Image.String(),
		ImagePullPolicy: mariadb.Spec.Image.PullPolicy,
		Env:             buildStsEnv(mariadb),
		EnvFrom:         mariadb.Spec.EnvFrom,
		Ports: []v1.ContainerPort{
			{
				ContainerPort: mariadb.Spec.Port,
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      stsStorageVolume,
				MountPath: stsStorageMountPath,
			},
		},
	}

	if mariadb.Spec.Resources != nil {
		container.Resources = *mariadb.Spec.Resources
	}

	return []v1.Container{container}, nil
}

func buildStsEnv(mariadb *databasev1alpha1.MariaDB) []v1.EnvVar {
	env := []v1.EnvVar{
		{
			Name:  "MYSQL_TCP_PORT",
			Value: strconv.Itoa(int(mariadb.Spec.Port)),
		},
		{
			Name: "MARIADB_ROOT_PASSWORD",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &mariadb.Spec.RootPasswordSecretKeyRef,
			},
		},
	}

	if mariadb.Spec.Database != nil {
		env = append(env, v1.EnvVar{
			Name:  "MARIADB_DATABASE",
			Value: *mariadb.Spec.Database,
		})
	}

	if mariadb.Spec.Username != nil {
		env = append(env, v1.EnvVar{
			Name:  "MARIADB_USER",
			Value: *mariadb.Spec.Username,
		})
	}

	if mariadb.Spec.PasswordSecretKeyRef != nil {
		env = append(env, v1.EnvVar{
			Name: "MARIADB_PASSWORD",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: mariadb.Spec.PasswordSecretKeyRef,
			},
		})
	}

	if mariadb.Spec.Env != nil {
		env = append(env, mariadb.Spec.Env...)
	}

	return env
}
