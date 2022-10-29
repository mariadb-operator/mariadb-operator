package builder

import (
	"errors"
	"fmt"
	"strings"

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
	batchStorageVolume    = "storage"
	batchStorageMountPath = "/data"
)

func (b *Builder) BuildBackupJob(key types.NamespacedName, backup *databasev1alpha1.BackupMariaDB,
	mariaDB *databasev1alpha1.MariaDB) (*batchv1.Job, error) {
	backupLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariaDB.Name).
			Build()
	meta := metav1.ObjectMeta{
		Name:      key.Name,
		Namespace: key.Namespace,
		Labels:    backupLabels,
	}
	backupFile := fmt.Sprintf(
		"%s/backup.$(date -u +'%s').sql",
		batchStorageMountPath,
		"%Y-%m-%dT%H:%M:%SZ",
	)
	cmds := []string{
		"echo '💾 Taking backup'",
		fmt.Sprintf(
			"mysqldump -h %s -P %d --lock-tables --all-databases > %s",
			mariaDB.Name,
			mariaDB.Spec.Port,
			backupFile,
		),
		"echo '🧹 Cleaning up old backups'",
		fmt.Sprintf(
			"find %s -name *.sql -type f -mtime +%d -exec rm {} ';'",
			batchStorageMountPath,
			backup.Spec.MaxRetentionDays,
		),
		"echo '📜 Backup history (last 10):'",
		fmt.Sprintf(
			"find %s -name *.sql -type f -printf '%s' | sort | tail -n 10",
			batchStorageMountPath,
			"%f\n",
		),
	}
	cmd := strings.Join(cmds, ";")

	opts := []jobOption{
		withJobMeta(meta),
		withJobVolumes(
			jobVolumes(backup),
		),
		withJobContainers(
			jobContainers(mariaDB, cmd, backup.Spec.Resources),
		),
		withJobBackoffLimit(backup.Spec.BackoffLimit),
		withJobRestartPolicy(backup.Spec.RestartPolicy),
	}
	if backup.Spec.MariaDBRef.WaitForIt {
		opts = addJobInitContainersOpt(mariaDB, opts)
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

func (b *Builder) BuildBackupCronJob(key types.NamespacedName, backup *databasev1alpha1.BackupMariaDB,
	mariaDB *databasev1alpha1.MariaDB) (*batchv1.CronJob, error) {
	if backup.Spec.Schedule == nil {
		return nil, errors.New("schedule field is mandatory when building a CronJob")
	}

	job, err := b.BuildBackupJob(key, backup, mariaDB)
	if err != nil {
		return nil, fmt.Errorf("error building BackupMariaDB: %v", err)
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          backup.Spec.Schedule.Cron,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			Suspend:           &backup.Spec.Schedule.Supend,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: job.ObjectMeta,
				Spec:       job.Spec,
			},
		},
	}
	if err := controllerutil.SetControllerReference(backup, cronJob, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to CronJob: %v", err)
	}
	return cronJob, nil
}

func (b *Builder) BuildRestoreJob(key types.NamespacedName, restore *databasev1alpha1.RestoreMariaDB,
	backup *databasev1alpha1.BackupMariaDB, mariaDB *databasev1alpha1.MariaDB) (*batchv1.Job, error) {
	restoreLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariaDB.Name).
			Build()
	meta := metav1.ObjectMeta{
		Name:      key.Name,
		Namespace: key.Namespace,
		Labels:    restoreLabels,
	}
	mostRecentBackup := fmt.Sprintf(
		"find %s -name *.sql -type f -printf '%s' | sort | tail -n 1",
		batchStorageMountPath,
		"%f\n",
	)
	cmds := []string{
		fmt.Sprintf(
			"export LATEST_BACKUP=%s/$(%s)",
			batchStorageMountPath,
			mostRecentBackup,
		),
		"echo '💾 Restoring most recent backup: '$LATEST_BACKUP''",
		fmt.Sprintf(
			"mysql -h %s -P %d < $LATEST_BACKUP",
			mariaDB.Name,
			mariaDB.Spec.Port,
		),
	}
	cmd := strings.Join(cmds, ";")

	opts := []jobOption{
		withJobMeta(meta),
		withJobVolumes(
			jobVolumes(backup),
		),
		withJobContainers(
			jobContainers(mariaDB, cmd, backup.Spec.Resources),
		),
		withJobBackoffLimit(backup.Spec.BackoffLimit),
		withJobRestartPolicy(backup.Spec.RestartPolicy),
	}
	if restore.Spec.MariaDBRef.WaitForIt {
		opts = addJobInitContainersOpt(mariaDB, opts)
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
	var volumeSource corev1.VolumeSource
	if backup.Spec.Storage.Volume != nil {
		volumeSource = *backup.Spec.Storage.Volume
	}
	if backup.Spec.Storage.PersistentVolumeClaim != nil {
		volumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: backup.Name,
			},
		}
	}

	return []corev1.Volume{
		{
			Name:         batchStorageVolume,
			VolumeSource: volumeSource,
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
				Name:      batchStorageVolume,
				MountPath: batchStorageMountPath,
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
