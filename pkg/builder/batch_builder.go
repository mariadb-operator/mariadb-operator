package builder

import (
	"errors"
	"fmt"
	"path/filepath"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	batchStorageVolume     = "backup"
	batchStorageMountPath  = "/backup"
	batchScriptsVolume     = "scripts"
	batchS3PKI             = "s3-pki"
	batchS3PKIMountPath    = "/s3/pki"
	batchScriptsMountPath  = "/opt"
	batchScriptsSqlFile    = "job.sql"
	batchUserEnv           = "MARIADB_OPERATOR_USER"
	batchPasswordEnv       = "MARIADB_OPERATOR_PASSWORD"
	batchS3AccessKeyId     = "AWS_ACCESS_KEY_ID"
	batchS3SecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	batchS3SessionTokenKey = "AWS_SESSION_TOKEN"
)

var batchBackupTargetFilePath = fmt.Sprintf("%s/0-backup-target.txt", batchStorageMountPath)

func (b *Builder) BuildBackupJob(key types.NamespacedName, backup *mariadbv1alpha1.Backup,
	mariadb *mariadbv1alpha1.MariaDB) (*batchv1.Job, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(backup.Spec.InheritMetadata).
			Build()

	cmdOpts := []command.BackupOpt{
		command.WithBackup(
			batchStorageMountPath,
			batchBackupTargetFilePath,
		),
		command.WithBackupMaxRetention(backup.Spec.MaxRetention.Duration),
		command.WithBackupUserEnv(batchUserEnv),
		command.WithBackupPasswordEnv(batchPasswordEnv),
		command.WithBackupLogLevel(backup.Spec.LogLevel),
		command.WithBackupDumpOpts(backup.Spec.Args),
	}
	cmdOpts = append(cmdOpts, s3Opts(backup.Spec.Storage.S3)...)

	cmd, err := command.NewBackupCommand(cmdOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building backup command: %v", err)
	}

	volume, err := backup.Volume()
	if err != nil {
		return nil, fmt.Errorf("error getting volume from Backup: %v", err)
	}
	volumes, volumeSources := jobBatchStorageVolume(volume, backup.Spec.Storage.S3)
	affinity := ptr.Deref(backup.Spec.Affinity, mariadbv1alpha1.AffinityConfig{}).Affinity

	job := &batchv1.Job{
		ObjectMeta: objMeta,
		Spec: batchv1.JobSpec{
			BackoffLimit: &backup.Spec.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: objMeta,
				Spec: corev1.PodSpec{
					RestartPolicy:    backup.Spec.RestartPolicy,
					ImagePullSecrets: batchImagePullSecrets(mariadb, backup.Spec.ImagePullSecrets),
					Volumes:          volumes,
					InitContainers: []corev1.Container{
						jobMariadbContainer(
							cmd.MariadbDump(backup, mariadb),
							volumeSources,
							jobEnv(mariadb),
							backup.Spec.Resources,
							mariadb,
							backup.Spec.SecurityContext,
						),
					},
					Containers: []corev1.Container{
						jobMariadbOperatorContainer(
							cmd.MariadbOperatorBackup(),
							volumeSources,
							jobS3Env(backup.Spec.Storage.S3),
							backup.Spec.Resources,
							mariadb,
							b.env,
							backup.Spec.SecurityContext,
						),
					},
					Affinity:           &affinity,
					NodeSelector:       backup.Spec.NodeSelector,
					Tolerations:        backup.Spec.Tolerations,
					SecurityContext:    backup.Spec.PodSecurityContext,
					ServiceAccountName: ptr.Deref(backup.Spec.ServiceAccountName, "default"),
				},
			},
		},
	}
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
			WithMetadata(backup.Spec.InheritMetadata).
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
			WithMetadata(restore.Spec.InheritMetadata).
			Build()
	cmdOpts := []command.BackupOpt{
		command.WithBackup(
			batchStorageMountPath,
			batchBackupTargetFilePath,
		),
		command.WithBackupTargetTime(restore.Spec.RestoreSource.TargetRecoveryTimeOrDefault()),
		command.WithBackupUserEnv(batchUserEnv),
		command.WithBackupPasswordEnv(batchPasswordEnv),
		command.WithBackupLogLevel(restore.Spec.LogLevel),
		command.WithBackupDumpOpts(restore.Spec.Args),
	}
	cmdOpts = append(cmdOpts, s3Opts(restore.Spec.S3)...)

	cmd, err := command.NewBackupCommand(cmdOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building restore command: %v", err)
	}
	volumes, volumeSources := jobBatchStorageVolume(restore.Spec.RestoreSource.Volume, restore.Spec.S3)
	affinity := ptr.Deref(restore.Spec.Affinity, mariadbv1alpha1.AffinityConfig{}).Affinity

	job := &batchv1.Job{
		ObjectMeta: objMeta,
		Spec: batchv1.JobSpec{
			BackoffLimit: &restore.Spec.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: objMeta,
				Spec: corev1.PodSpec{
					RestartPolicy:    restore.Spec.RestartPolicy,
					ImagePullSecrets: batchImagePullSecrets(mariadb, restore.Spec.ImagePullSecrets),
					Volumes:          volumes,
					InitContainers: []corev1.Container{
						jobMariadbOperatorContainer(
							cmd.MariadbOperatorRestore(),
							volumeSources,
							jobS3Env(restore.Spec.S3),
							restore.Spec.Resources,
							mariadb,
							b.env,
							restore.Spec.SecurityContext,
						),
					},
					Containers: []corev1.Container{
						jobMariadbContainer(
							cmd.MariadbRestore(mariadb),
							volumeSources,
							jobEnv(mariadb),
							restore.Spec.Resources,
							mariadb,
							restore.Spec.SecurityContext,
						),
					},
					Affinity:           &affinity,
					NodeSelector:       restore.Spec.NodeSelector,
					Tolerations:        restore.Spec.Tolerations,
					SecurityContext:    restore.Spec.PodSecurityContext,
					ServiceAccountName: ptr.Deref(restore.Spec.ServiceAccountName, "default"),
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(restore, job, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Job: %v", err)
	}
	return job, nil
}

func (b *Builder) BuilInitJob(key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB,
	meta *mariadbv1alpha1.Metadata) (*batchv1.Job, error) {
	extraMeta := ptr.Deref(meta, mariadbv1alpha1.Metadata{})
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(mariadb.Spec.InheritMetadata).
			WithMetadata(&extraMeta).
			Build()
	command := command.NewBashCommand([]string{
		filepath.Join(InitConfigPath, InitEntrypointKey),
	})

	podTpl, err := b.mariadbPodTemplate(
		mariadb,
		withMeta(mariadb.Spec.InheritMetadata),
		withMeta(&extraMeta),
		withCommand(command.Command),
		withArgs(command.Args),
		withRestartPolicy(corev1.RestartPolicyOnFailure),
		withExtraVolumes([]corev1.Volume{
			{
				Name: StorageVolume,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: mariadb.PVCKey(StorageVolume, 0).Name,
					},
				},
			},
			{
				Name: InitVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: mariadb.InitKey().Name,
						},
						DefaultMode: ptr.To(int32(0777)),
					},
				},
			},
		}),
		withExtraVolumeMounts([]corev1.VolumeMount{
			{
				Name:      InitVolume,
				MountPath: InitConfigPath,
			},
		}),
		withGalera(false),
		withPorts(false),
		withProbes(false),
	)
	if err != nil {
		return nil, err
	}

	job := &batchv1.Job{
		ObjectMeta: objMeta,
		Spec: batchv1.JobSpec{
			Template: *podTpl,
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, job, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Job: %v", err)
	}
	return job, nil
}

func (b *Builder) BuildSqlJob(key types.NamespacedName, sqlJob *mariadbv1alpha1.SqlJob,
	mariadb *mariadbv1alpha1.MariaDB) (*batchv1.Job, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(sqlJob.Spec.InheritMetadata).
			Build()

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

	volumes, volumeMounts := sqlJobvolumes(sqlJob)
	affinity := ptr.Deref(sqlJob.Spec.Affinity, mariadbv1alpha1.AffinityConfig{}).Affinity

	job := &batchv1.Job{
		ObjectMeta: objMeta,
		Spec: batchv1.JobSpec{
			BackoffLimit: &sqlJob.Spec.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: objMeta,
				Spec: corev1.PodSpec{
					RestartPolicy:    sqlJob.Spec.RestartPolicy,
					ImagePullSecrets: batchImagePullSecrets(mariadb, sqlJob.Spec.ImagePullSecrets),
					Volumes:          volumes,
					Containers: []corev1.Container{
						jobMariadbContainer(
							cmd.ExecCommand(mariadb),
							volumeMounts,
							sqlJobEnv(sqlJob),
							sqlJob.Spec.Resources,
							mariadb,
							sqlJob.Spec.SecurityContext,
						),
					},
					Affinity:           &affinity,
					NodeSelector:       sqlJob.Spec.NodeSelector,
					Tolerations:        sqlJob.Spec.Tolerations,
					SecurityContext:    sqlJob.Spec.PodSecurityContext,
					ServiceAccountName: ptr.Deref(sqlJob.Spec.ServiceAccountName, "default"),
				},
			},
		},
	}
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
			WithMetadata(sqlJob.Spec.InheritMetadata).
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

func s3Opts(s3 *mariadbv1alpha1.S3) []command.BackupOpt {
	if s3 == nil {
		return nil
	}
	tls := ptr.Deref(s3.TLS, mariadbv1alpha1.TLS{})

	cmdOpts := []command.BackupOpt{
		command.WithS3(
			s3.Bucket,
			s3.Endpoint,
			s3.Region,
			s3.Prefix,
		),
		command.WithS3TLS(tls.Enabled),
	}
	if tls.Enabled && tls.CASecretKeyRef != nil {
		caCertPath := filepath.Join(batchS3PKIMountPath, s3.TLS.CASecretKeyRef.Key)
		cmdOpts = append(cmdOpts, command.WithS3CACertPath(caCertPath))
	}
	return cmdOpts
}

func batchImagePullSecrets(mariadb *mariadbv1alpha1.MariaDB, pullSecrets []corev1.LocalObjectReference) []corev1.LocalObjectReference {
	var secrets []corev1.LocalObjectReference
	secrets = append(secrets, mariadb.Spec.ImagePullSecrets...)
	secrets = append(secrets, pullSecrets...)
	return secrets
}
