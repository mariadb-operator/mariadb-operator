package builder

import (
	"errors"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	batchStorageVolume    = "backup"
	batchStorageMountPath = "/backup"
	batchScriptsVolume    = "scripts"
	batchScriptsMountPath = "/opt"
	batchScriptsSqlFile   = "job.sql"
	batchUserEnv          = "MARIADB_OPERATOR_USER"
	batchPasswordEnv      = "MARIADB_OPERATOR_PASSWORD"
)

var batchBackupTargetFilePath = fmt.Sprintf("%s/0-backup-target.txt", batchStorageMountPath)

func (b *Builder) BuildBackupJob(key types.NamespacedName, backup *mariadbv1alpha1.Backup,
	mariadb *mariadbv1alpha1.MariaDB) (*batchv1.Job, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()

	cmdOpts := []command.BackupOpt{
		command.WithBackup(
			batchStorageMountPath,
			batchBackupTargetFilePath,
			time.Now(),
		),
		command.WithBackupUserEnv(batchUserEnv),
		command.WithBackupPasswordEnv(batchPasswordEnv),
	}
	if backup.Spec.Args != nil {
		cmdOpts = append(cmdOpts, command.WithBackupDumpOpts(backup.Spec.Args))
	}
	cmd, err := command.NewBackupCommand(cmdOpts...)
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
		withJobVolumes(volumes...),
		withJobContainers(
			jobMariadbContainer(
				cmd.MariadbDump(backup, mariadb),
				volumeSources,
				jobEnv(mariadb),
				backup.Spec.Resources,
				mariadb,
			),
		),
		withJobBackoffLimit(backup.Spec.BackoffLimit),
		withJobRestartPolicy(backup.Spec.RestartPolicy),
		withAffinity(backup.Spec.Affinity),
		withNodeSelector(backup.Spec.NodeSelector),
		withTolerations(backup.Spec.Tolerations...),
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
			Suspend:           &backup.Spec.Schedule.Suspend,
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
	cmdOpts := []command.BackupOpt{
		command.WithBackup(
			batchStorageMountPath,
			batchBackupTargetFilePath,
			restore.Spec.RestoreSource.TargetRecoveryTimeOrDefault(),
		),
		command.WithBackupUserEnv(batchUserEnv),
		command.WithBackupPasswordEnv(batchPasswordEnv),
	}
	cmd, err := command.NewBackupCommand(cmdOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building restore command: %v", err)
	}
	volumes, volumeSources := jobBatchStorageVolume(restore.Spec.RestoreSource.Volume)

	jobOpts := []jobOption{
		withJobMeta(objMeta),
		withJobVolumes(volumes...),
		withJobInitContainers(
			jobMariadbOperatorContainer(
				cmd.MariadbOperatorRestore(),
				volumeSources,
				restore.Spec.Resources,
				mariadb,
				b.env,
			),
		),
		withJobContainers(
			jobMariadbContainer(
				cmd.MariadbRestore(mariadb),
				volumeSources,
				jobEnv(mariadb),
				restore.Spec.Resources,
				mariadb,
			),
		),
		withJobBackoffLimit(restore.Spec.BackoffLimit),
		withJobRestartPolicy(restore.Spec.RestartPolicy),
		withAffinity(restore.Spec.Affinity),
		withNodeSelector(restore.Spec.NodeSelector),
		withTolerations(restore.Spec.Tolerations...),
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

	sqlOpts := []command.SqlOpt{
		command.WithSqlUserEnv(batchUserEnv),
		command.WithSqlPasswordEnv(batchPasswordEnv),
		command.WithSqlFile(fmt.Sprintf("%s/%s", batchScriptsMountPath, batchScriptsSqlFile)),
	}
	if sqlJob.Spec.Database != nil {
		sqlOpts = append(sqlOpts, command.WithSqlDatabase(*sqlJob.Spec.Database))
	}
	cmd, err := command.NewSqlCommand(sqlOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building sql command: %v", err)
	}

	jobOpts := []jobOption{
		withJobMeta(objMeta),
		withJobVolumes(volumes...),
		withJobContainers(
			jobMariadbContainer(
				cmd.ExecCommand(mariadb),
				volumeMounts,
				sqlJobEnv(sqlJob),
				sqlJob.Spec.Resources,
				mariadb,
			),
		),
		withJobBackoffLimit(sqlJob.Spec.BackoffLimit),
		withJobRestartPolicy(sqlJob.Spec.RestartPolicy),
		withAffinity(sqlJob.Spec.Affinity),
		withNodeSelector(sqlJob.Spec.NodeSelector),
		withTolerations(sqlJob.Spec.Tolerations...),
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

func (b *Builder) BuildSqlCronJob(key types.NamespacedName, sqlJob *mariadbv1alpha1.SqlJob,
	mariadb *mariadbv1alpha1.MariaDB) (*batchv1.CronJob, error) {
	if sqlJob.Spec.Schedule == nil {
		return nil, errors.New("schedule field is mandatory when building a CronJob")
	}

	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	job, err := b.BuildSqlJob(key, sqlJob, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error building SqlJob: %v", err)
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: objMeta,
		Spec: batchv1.CronJobSpec{
			Schedule:          sqlJob.Spec.Schedule.Cron,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			Suspend:           &sqlJob.Spec.Schedule.Suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: job.ObjectMeta,
				Spec:       job.Spec,
			},
		},
	}
	if err := controllerutil.SetControllerReference(sqlJob, cronJob, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to CronJob: %v", err)
	}
	return cronJob, nil
}
