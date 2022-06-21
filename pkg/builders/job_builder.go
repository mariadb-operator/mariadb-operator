package builders

import (
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	jobStorageVolume    = "storage"
	jobStorageMountPath = "/data"
)

var (
	dumpFilePath = fmt.Sprintf("%s/backup.sql", jobStorageMountPath)
)

func BuildJob(backup *databasev1alpha1.BackupMariaDB, mariadb *databasev1alpha1.MariaDB) *batchv1.Job {
	labels := NewLabelsBuilder().WithObjectMeta(backup.ObjectMeta).WithApp(app).Build()

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backup.Spec.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backup.Name,
					Namespace: backup.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					Volumes:       buildJobVolumes(backup),
					Containers:    buildJobContainers(backup, mariadb),
					RestartPolicy: backup.Spec.RestartPolicy,
				},
			},
		},
	}
}

func buildJobVolumes(backup *databasev1alpha1.BackupMariaDB) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: jobStorageVolume,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: backup.Name,
				},
			},
		},
	}
}

func buildJobContainers(backup *databasev1alpha1.BackupMariaDB, mariadb *databasev1alpha1.MariaDB) []corev1.Container {
	image := fmt.Sprintf("%s:%s", mariadb.Spec.Image.Repository, mariadb.Spec.Image.Tag)
	cmd := fmt.Sprintf(
		"mysqldump -h %s -P %d --lock-tables --all-databases > %s",
		mariadb.Name,
		mariadb.Spec.Port,
		dumpFilePath,
	)

	return []corev1.Container{
		{
			Name:            backup.Name,
			Image:           image,
			ImagePullPolicy: mariadb.Spec.Image.PullPolicy,
			Command:         []string{"sh", "-c"},
			Args:            []string{cmd},
			Env:             builJobEnv(mariadb),
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      jobStorageVolume,
					MountPath: jobStorageMountPath,
				},
			},
			Resources: *backup.Spec.Resources,
		},
	}
}

func builJobEnv(mariadb *databasev1alpha1.MariaDB) []v1.EnvVar {
	return []v1.EnvVar{
		{
			Name:  "USER",
			Value: "root",
		},
		{
			Name: "MYSQL_PWD",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &mariadb.Spec.RootPasswordSecretKeyRef,
			},
		},
	}
}
