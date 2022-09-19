package builder

import (
	"errors"
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	labels "github.com/mmontes11/mariadb-operator/pkg/builder/labels"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	jobStorageVolume    = "storage"
	jobStorageMountPath = "/data"
)

var (
	dumpFilePath = fmt.Sprintf("%s/backup.sql", jobStorageMountPath)
)

func (b *Builder) BuildBackupJob(backup *databasev1alpha1.BackupMariaDB, mariadb *databasev1alpha1.MariaDB,
	key types.NamespacedName) (*batchv1.Job, error) {
	backupLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			Build()
	meta := metav1.ObjectMeta{
		Name:      key.Name,
		Namespace: key.Namespace,
		Labels:    backupLabels,
	}
	cmd := fmt.Sprintf(
		"mysqldump -h %s -P %d --lock-tables --all-databases > %s",
		mariadb.Name,
		mariadb.Spec.Port,
		dumpFilePath,
	)

	opts := []jobOption{
		withJobMeta(meta),
		withJobVolumes(
			jobVolumes(backup),
		),
		withJobContainers(
			jobContainers(mariadb, cmd, backup.Spec.Resources),
		),
		withJobBackoffLimit(backup.Spec.BackoffLimit),
		withJobRestartPolicy(backup.Spec.RestartPolicy),
	}
	if backup.Spec.WaitForMariaDB {
		opts = addJobInitContainersOpt(mariadb, opts)
	}

	builder, err := newJobBuilder(opts...)
	if err != nil {
		return nil, fmt.Errorf("error building backup Job: %v", err)
	}

	job := builder.build()
	if err := controllerutil.SetControllerReference(backup, job, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Job: %v", err)
	}
	return job, nil
}

func (b *Builder) BuildRestoreJob(restore *databasev1alpha1.RestoreMariaDB, mariadb *databasev1alpha1.MariaDB,
	backup *databasev1alpha1.BackupMariaDB, key types.NamespacedName) (*batchv1.Job, error) {
	restoreLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			Build()
	meta := metav1.ObjectMeta{
		Name:      key.Name,
		Namespace: key.Namespace,
		Labels:    restoreLabels,
	}
	cmd := fmt.Sprintf(
		"mysql -h %s -P %d < %s",
		mariadb.Name,
		mariadb.Spec.Port,
		dumpFilePath,
	)

	opts := []jobOption{
		withJobMeta(meta),
		withJobVolumes(
			jobVolumes(backup),
		),
		withJobContainers(
			jobContainers(mariadb, cmd, backup.Spec.Resources),
		),
		withJobBackoffLimit(backup.Spec.BackoffLimit),
		withJobRestartPolicy(backup.Spec.RestartPolicy),
	}
	if restore.Spec.WaitForMariaDB {
		opts = addJobInitContainersOpt(mariadb, opts)
	}

	builder, err := newJobBuilder(opts...)
	if err != nil {
		return nil, fmt.Errorf("error building backup Job: %v", err)
	}

	job := builder.build()
	if err := controllerutil.SetControllerReference(restore, job, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Job: %v", err)
	}
	return job, nil
}

func addJobInitContainersOpt(mariadb *databasev1alpha1.MariaDB, opts []jobOption) []jobOption {
	initCmd := fmt.Sprintf(
		"while ! mysqladmin ping -h %s -P %d --protocol tcp --silent; do echo 'waiting for mariadb...'; sleep 1s; done",
		mariadb.Name,
		mariadb.Spec.Port,
	)
	return append(opts,
		withJobInitContainers(
			jobInitContainers(mariadb, initCmd),
		),
	)
}

type jobOption func(*jobBuilder)

func withJobMeta(meta metav1.ObjectMeta) jobOption {
	return func(b *jobBuilder) {
		b.meta = &meta
	}
}

func withJobVolumes(volumes []corev1.Volume) jobOption {
	return func(b *jobBuilder) {
		b.volumes = volumes
	}
}

func withJobInitContainers(initContainers []corev1.Container) jobOption {
	return func(b *jobBuilder) {
		b.initContainers = initContainers
	}
}

func withJobContainers(containers []v1.Container) jobOption {
	return func(b *jobBuilder) {
		b.containers = containers
	}
}

func withJobBackoffLimit(backoffLimit int32) jobOption {
	return func(b *jobBuilder) {
		b.backoffLimit = &backoffLimit
	}
}

func withJobRestartPolicy(restartPolicy corev1.RestartPolicy) jobOption {
	return func(b *jobBuilder) {
		b.restartPolicy = &restartPolicy
	}
}

type jobBuilder struct {
	meta           *metav1.ObjectMeta
	volumes        []corev1.Volume
	initContainers []corev1.Container
	containers     []corev1.Container
	backoffLimit   *int32
	restartPolicy  *corev1.RestartPolicy
}

func newJobBuilder(opts ...jobOption) (*jobBuilder, error) {
	builder := jobBuilder{}
	for _, setOpt := range opts {
		setOpt(&builder)
	}

	if builder.meta == nil {
		return nil, errors.New("meta is mandatory")
	}
	if builder.volumes == nil {
		return nil, errors.New("volumes are mandatory")
	}
	if builder.containers == nil {
		return nil, errors.New("containers are mandatory")
	}
	return &builder, nil
}

func (b *jobBuilder) build() *batchv1.Job {
	template := corev1.PodTemplateSpec{
		ObjectMeta: *b.meta,
		Spec: corev1.PodSpec{
			Volumes:    b.volumes,
			Containers: b.containers,
		},
	}
	if b.initContainers != nil {
		template.Spec.InitContainers = b.initContainers
	}
	if b.restartPolicy != nil {
		template.Spec.RestartPolicy = *b.restartPolicy
	}

	job := &batchv1.Job{
		ObjectMeta: *b.meta,
		Spec: batchv1.JobSpec{
			Template: template,
		},
	}
	if b.backoffLimit != nil {
		job.Spec.BackoffLimit = b.backoffLimit
	}
	return job
}

func jobVolumes(backup *databasev1alpha1.BackupMariaDB) []corev1.Volume {
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

func jobInitContainers(mariadb *databasev1alpha1.MariaDB, cmd string) []corev1.Container {
	return []corev1.Container{
		{
			Name:            "wait-for-mariadb",
			Image:           mariadb.Spec.Image.String(),
			ImagePullPolicy: mariadb.Spec.Image.PullPolicy,
			Command:         []string{"sh", "-c"},
			Args:            []string{cmd},
			Env:             jobEnv(mariadb),
		},
	}
}

func jobContainers(mariadb *databasev1alpha1.MariaDB, cmd string, resources *corev1.ResourceRequirements) []corev1.Container {
	container := corev1.Container{
		Name:            "mariadb",
		Image:           mariadb.Spec.Image.String(),
		ImagePullPolicy: mariadb.Spec.Image.PullPolicy,
		Command:         []string{"sh", "-c"},
		Args:            []string{cmd},
		Env:             jobEnv(mariadb),
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

func jobEnv(mariadb *databasev1alpha1.MariaDB) []v1.EnvVar {
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
