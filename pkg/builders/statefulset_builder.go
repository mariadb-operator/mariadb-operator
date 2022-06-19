package builders

import (
	"context"
	"fmt"
	"strconv"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/defaulter"
	"github.com/mmontes11/mariadb-operator/pkg/refreader"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	app                  = "mariadb"
	storageVolume        = "storage"
	storageMountPath     = "/var/lib/mysql"
	defaultImageTag      = "latest"
	defaultContainerPort = 3306
)

type StatefulSetBuilder struct {
	refReader *refreader.RefReader
}

func NewStatefulSetBuilder(refReader *refreader.RefReader) *StatefulSetBuilder {
	return &StatefulSetBuilder{
		refReader: refReader,
	}
}

func (b *StatefulSetBuilder) Build(ctx context.Context, mariadb *databasev1alpha1.MariaDB) (*appsv1.StatefulSet, error) {
	labels := NewLabelsBuilder().WithObjectMeta(mariadb.ObjectMeta).WithApp(app).Build()
	containers, err := b.buildContainers(ctx, mariadb)
	if err != nil {
		return nil, err
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.Name,
			Namespace: mariadb.Namespace,
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
			VolumeClaimTemplates: b.buildVolumeClaimTemplates(mariadb),
		},
	}, nil
}

func (b *StatefulSetBuilder) buildContainers(ctx context.Context, mariadb *databasev1alpha1.MariaDB) ([]v1.Container, error) {
	image := fmt.Sprintf("%s:%s",
		mariadb.Spec.Image.Repository,
		defaulter.String(mariadb.Spec.Image.Tag, defaultImageTag),
	)
	env, err := b.buildEnv(ctx, mariadb)
	if err != nil {
		return nil, err
	}

	container := v1.Container{
		Name:            mariadb.Name,
		Image:           image,
		ImagePullPolicy: defaulter.PullPolicy(mariadb.Spec.Image.PullPolicy, corev1.PullIfNotPresent),
		Env:             env,
		EnvFrom:         mariadb.Spec.EnvFrom,
		Ports: []v1.ContainerPort{
			{
				ContainerPort: defaulter.Int32(mariadb.Spec.Port, defaultContainerPort),
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      storageVolume,
				MountPath: storageMountPath,
			},
		},
	}

	if mariadb.Spec.Resources != nil {
		container.Resources = *mariadb.Spec.Resources
	}

	return []v1.Container{container}, nil
}

func (b *StatefulSetBuilder) buildEnv(ctx context.Context, mariadb *databasev1alpha1.MariaDB) ([]v1.EnvVar, error) {
	rootPasswd, err := b.refReader.ReadSecretKeyRef(ctx, mariadb.Spec.RootPasswordSecretKeyRef, mariadb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading root password secret: %v", err)
	}

	env := []v1.EnvVar{
		{
			Name:  "MYSQL_TCP_PORT",
			Value: strconv.Itoa(int(defaulter.Int32(mariadb.Spec.Port, defaultContainerPort))),
		},
		{
			Name:  "MARIADB_ROOT_PASSWORD",
			Value: rootPasswd,
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
		passwd, err := b.refReader.ReadSecretKeyRef(ctx, *mariadb.Spec.PasswordSecretKeyRef, mariadb.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error reading password secret: %v", err)
		}
		env = append(env, v1.EnvVar{
			Name:  "MARIADB_PASSWORD",
			Value: passwd,
		})
	}

	for _, e := range mariadb.Spec.Env {
		env = append(env, e)
	}

	return env, nil
}

func (b *StatefulSetBuilder) buildVolumeClaimTemplates(mariadb *databasev1alpha1.MariaDB) []v1.PersistentVolumeClaim {
	accessModes := mariadb.Spec.Storage.AccessModes
	if accessModes == nil {
		accessModes = []v1.PersistentVolumeAccessMode{
			v1.ReadWriteOnce,
		}
	}

	return []v1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageVolume,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes:      accessModes,
				StorageClassName: &mariadb.Spec.Storage.ClassName,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: mariadb.Spec.Storage.Size,
					},
				},
			},
		},
	}
}
