package builder

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	cmd "github.com/mariadb-operator/mariadb-operator/pkg/command"
	backupcmd "github.com/mariadb-operator/mariadb-operator/pkg/command/backup"
	sqlcmd "github.com/mariadb-operator/mariadb-operator/pkg/command/sql"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	batchDataVolume         = "data"
	batchDataMountPath      = "/var/lib/mysql"
	batchStorageVolume      = "backup"
	batchStorageMountPath   = "/backup"
	batchScriptsVolume      = "scripts"
	batchScriptsMountPath   = "/opt"
	batchScriptsSqlFileName = "job.sql"
	batchUserEnv            = "MARIADB_OPERATOR_USER"
	batchPasswordEnv        = "MARIADB_OPERATOR_PASSWORD"
)

func (b *Builder) BuildBackupJob(key types.NamespacedName, backup *mariadbv1alpha1.Backup,
	mariadb *mariadbv1alpha1.MariaDB) (*batchv1.Job, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()

	cmdOpts := []backupcmd.Option{
		backupcmd.WithBasePath(batchStorageMountPath),
		backupcmd.WithUserEnv(batchUserEnv),
		backupcmd.WithPasswordEnv(batchPasswordEnv),
	}
	cmd, err := backupcmd.New(cmdOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building backup command: %v", err)
	}

	volume, err := backup.Volume()
	if err != nil {
		return nil, fmt.Errorf("error getting volume from Backup: %v", err)
	}
	volumes, volumeSources := jobBatchStorageVolume(volume)

	opts := []jobOption{
		withJobMeta(objMeta),
		withJobVolumes(volumes),
		withJobContainers(
			jobContainers(
				cmd.BackupCommand(backup, mariadb),
				jobEnv(mariadb),
				volumeSources,
				backup.Spec.Resources,
				mariadb,
			),
		),
		withJobBackoffLimit(backup.Spec.BackoffLimit),
		withJobRestartPolicy(backup.Spec.RestartPolicy),
		withAffinity(backup.Spec.Affinity),
		withNodeSelector(backup.Spec.NodeSelector),
		withTolerations(backup.Spec.Tolerations),
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

func (b *Builder) BuildBackupCronJob(key types.NamespacedName, backup *mariadbv1alpha1.Backup,
	mariadb *mariadbv1alpha1.MariaDB) (*batchv1.CronJob, error) {
	if backup.Spec.Schedule == nil {
		return nil, errors.New("schedule field is mandatory when building a CronJob")
	}

	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	job, err := b.BuildBackupJob(key, backup, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error building Backup: %v", err)
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: objMeta,
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

func (b *Builder) BuildRestoreJob(key types.NamespacedName, restore *mariadbv1alpha1.Restore,
	mariadb *mariadbv1alpha1.MariaDB) (*batchv1.Job, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	cmdOpts := []backupcmd.Option{
		backupcmd.WithBasePath(batchStorageMountPath),
		backupcmd.WithUserEnv(batchUserEnv),
		backupcmd.WithPasswordEnv(batchPasswordEnv),
	}
	if restore.Spec.RestoreSource.FileName != nil {
		cmdOpts = append(cmdOpts, backupcmd.WithFile(*restore.Spec.RestoreSource.FileName))
	}
	cmd, err := backupcmd.New(cmdOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building restore command: %v", err)
	}
	volumes, volumeSources := jobBatchStorageVolume(restore.Spec.RestoreSource.Volume)

	jobOpts := []jobOption{
		withJobMeta(objMeta),
		withJobVolumes(volumes),
		withJobContainers(
			jobContainers(
				cmd.RestoreCommand(mariadb),
				jobEnv(mariadb),
				volumeSources,
				restore.Spec.Resources,
				mariadb,
			),
		),
		withJobBackoffLimit(restore.Spec.BackoffLimit),
		withJobRestartPolicy(restore.Spec.RestartPolicy),
		withAffinity(restore.Spec.Affinity),
		withNodeSelector(restore.Spec.NodeSelector),
		withTolerations(restore.Spec.Tolerations),
	}

	builder, err := newJobBuilder(jobOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building backup Job: %v", err)
	}

	job := builder.build()
	if err := controllerutil.SetControllerReference(restore, job, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Job: %v", err)
	}
	return job, nil
}

func (b *Builder) BuildSqlJob(key types.NamespacedName, sqlJob *mariadbv1alpha1.SqlJob,
	mariadb *mariadbv1alpha1.MariaDB) (*batchv1.Job, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()

	volumes, volumeMounts := sqlJobvolumes(sqlJob)

	sqlOpts := []sqlcmd.Option{
		sqlcmd.WithUserEnv(batchUserEnv),
		sqlcmd.WithPasswordEnv(batchPasswordEnv),
		sqlcmd.WithSqlFile(fmt.Sprintf("%s/%s", batchScriptsMountPath, batchScriptsSqlFileName)),
	}
	if sqlJob.Spec.Database != nil {
		sqlOpts = append(sqlOpts, sqlcmd.WithDatabase(*sqlJob.Spec.Database))
	}
	cmd, err := sqlcmd.New(sqlOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building sql command: %v", err)
	}

	jobOpts := []jobOption{
		withJobMeta(objMeta),
		withJobVolumes(volumes),
		withJobContainers(
			jobContainers(
				cmd.ExecCommand(mariadb),
				sqlJobEnv(sqlJob),
				volumeMounts,
				sqlJob.Spec.Resources,
				mariadb,
			),
		),
		withJobBackoffLimit(sqlJob.Spec.BackoffLimit),
		withJobRestartPolicy(sqlJob.Spec.RestartPolicy),
		withAffinity(sqlJob.Spec.Affinity),
		withNodeSelector(sqlJob.Spec.NodeSelector),
		withTolerations(sqlJob.Spec.Tolerations),
	}

	builder, err := newJobBuilder(jobOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building sql Job: %v", err)
	}

	job := builder.build()
	if err := controllerutil.SetControllerReference(sqlJob, job, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Job: %v", err)
	}
	return job, nil
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

func withAffinity(affinity *corev1.Affinity) jobOption {
	return func(b *jobBuilder) {
		b.affinity = affinity
	}
}

func withNodeSelector(nodeSelector map[string]string) jobOption {
	return func(b *jobBuilder) {
		b.nodeSelector = nodeSelector
	}
}

func withTolerations(tolerations []corev1.Toleration) jobOption {
	return func(b *jobBuilder) {
		b.tolerations = tolerations
	}
}

type jobBuilder struct {
	meta           *metav1.ObjectMeta
	volumes        []corev1.Volume
	initContainers []corev1.Container
	containers     []corev1.Container
	backoffLimit   *int32
	restartPolicy  *corev1.RestartPolicy
	affinity       *corev1.Affinity
	nodeSelector   map[string]string
	tolerations    []corev1.Toleration
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
			Volumes:      b.volumes,
			Containers:   b.containers,
			Affinity:     b.affinity,
			NodeSelector: b.nodeSelector,
			Tolerations:  b.tolerations,
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

func sqlJobvolumes(sqlJob *mariadbv1alpha1.SqlJob) ([]corev1.Volume, []corev1.VolumeMount) {
	return []corev1.Volume{
			{
				Name: batchScriptsVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: sqlJob.Spec.SqlConfigMapKeyRef.LocalObjectReference,
						Items: []corev1.KeyToPath{
							{
								Key:  sqlJob.Spec.SqlConfigMapKeyRef.Key,
								Path: batchScriptsSqlFileName,
							},
						},
					},
				},
			},
		}, []corev1.VolumeMount{
			{
				Name:      batchScriptsVolume,
				MountPath: batchScriptsMountPath,
			},
		}
}

func jobContainers(cmd *cmd.Command, env []v1.EnvVar, volumeMounts []corev1.VolumeMount,
	resources *corev1.ResourceRequirements, mariadb *mariadbv1alpha1.MariaDB) []corev1.Container {

	container := corev1.Container{
		Name:            "mariadb",
		Image:           mariadb.Spec.Image.String(),
		ImagePullPolicy: mariadb.Spec.Image.PullPolicy,
		Command:         cmd.Command,
		Args:            cmd.Args,
		Env:             env,
		VolumeMounts:    volumeMounts,
	}
	if resources != nil {
		container.Resources = *resources
	}
	return []corev1.Container{container}
}

func jobEnv(mariadb *mariadbv1alpha1.MariaDB) []v1.EnvVar {
	return []v1.EnvVar{
		{
			Name:  batchUserEnv,
			Value: "root",
		},
		{
			Name: batchPasswordEnv,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &mariadb.Spec.RootPasswordSecretKeyRef,
			},
		},
	}
}

func jobBatchStorageVolume(volumeSource *corev1.VolumeSource) ([]corev1.Volume, []corev1.VolumeMount) {
	return []corev1.Volume{
			{
				Name:         batchStorageVolume,
				VolumeSource: *volumeSource,
			},
		}, []corev1.VolumeMount{
			{
				Name:      batchStorageVolume,
				MountPath: batchStorageMountPath,
			},
		}
}

func sqlJobEnv(sqlJob *mariadbv1alpha1.SqlJob) []v1.EnvVar {
	return []v1.EnvVar{
		{
			Name:  batchUserEnv,
			Value: sqlJob.Spec.Username,
		},
		{
			Name: batchPasswordEnv,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &sqlJob.Spec.PasswordSecretKeyRef,
			},
		},
	}
}
