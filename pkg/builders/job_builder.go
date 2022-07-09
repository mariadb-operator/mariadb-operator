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

func BuildBackupJob(backup *databasev1alpha1.BackupMariaDB, mariadb *databasev1alpha1.MariaDB) *batchv1.Job {
	labels :=
		NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			Build()
	meta := metav1.ObjectMeta{
		Name:      backup.Name,
		Namespace: backup.Namespace,
		Labels:    labels,
	}
	volumes := buildJobVolumes(backup)
	cmd := fmt.Sprintf(
		"mysqldump -h %s -P %d --lock-tables --all-databases > %s",
		mariadb.Name,
		mariadb.Spec.Port,
		dumpFilePath,
	)
	containers := builJobContainers(mariadb, backup, cmd, backup.Spec.Resources)

	return buildJob(meta, volumes, containers, &backup.Spec.BackoffLimit, backup.Spec.RestartPolicy)
}

func BuildRestoreJob(restore *databasev1alpha1.RestoreMariaDB,
	mariadb *databasev1alpha1.MariaDB, backup *databasev1alpha1.BackupMariaDB) *batchv1.Job {
	labels :=
		NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			Build()
	meta := metav1.ObjectMeta{
		Name:      restore.Name,
		Namespace: restore.Namespace,
		Labels:    labels,
	}
	volumes := buildJobVolumes(backup)
	cmd := fmt.Sprintf(
		"mysql -h %s -P %d < %s",
		mariadb.Name,
		mariadb.Spec.Port,
		dumpFilePath,
	)
	containers := builJobContainers(mariadb, backup, cmd, restore.Spec.Resources)

	return buildJob(meta, volumes, containers, &backup.Spec.BackoffLimit, backup.Spec.RestartPolicy)
}

func buildJob(meta metav1.ObjectMeta, volumes []corev1.Volume, containers []corev1.Container,
	backoffLimit *int32, restartPolicy corev1.RestartPolicy) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: meta,
		Spec: batchv1.JobSpec{
			BackoffLimit: backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta,
				Spec: corev1.PodSpec{
					Volumes:       volumes,
					Containers:    containers,
					RestartPolicy: restartPolicy,
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

func builJobContainers(mariadb *databasev1alpha1.MariaDB, backup *databasev1alpha1.BackupMariaDB,
	cmd string, resources *corev1.ResourceRequirements) []corev1.Container {
	image := fmt.Sprintf("%s:%s", mariadb.Spec.Image.Repository, mariadb.Spec.Image.Tag)

	container := corev1.Container{
		Name:            mariadb.ObjectMeta.Name,
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
	}
	if resources != nil {
		container.Resources = *resources
	}

	return []corev1.Container{container}
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
