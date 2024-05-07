package builder

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	cmd "github.com/mariadb-operator/mariadb-operator/pkg/command"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

func (b *Builder) jobContainer(name string, cmd *cmd.Command, image string, volumeMounts []corev1.VolumeMount, env []v1.EnvVar,
	resources *corev1.ResourceRequirements, mariadb *mariadbv1alpha1.MariaDB,
	securityContext *corev1.SecurityContext) (*corev1.Container, error) {
	sc, err := b.buildContainerSecurityContext(securityContext)
	if err != nil {
		return nil, err
	}

	container := corev1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: mariadb.Spec.ImagePullPolicy,
		Command:         cmd.Command,
		Args:            cmd.Args,
		Env:             env,
		VolumeMounts:    volumeMounts,
		SecurityContext: sc,
	}
	if resources != nil {
		container.Resources = *resources
	}
	return &container, nil
}

func (b *Builder) jobMariadbOperatorContainer(cmd *cmd.Command, volumeMounts []corev1.VolumeMount, envVar []v1.EnvVar,
	resources *corev1.ResourceRequirements, mariadb *mariadbv1alpha1.MariaDB, env *environment.OperatorEnv,
	securityContext *corev1.SecurityContext) (*corev1.Container, error) {

	return b.jobContainer("mariadb-operator", cmd, env.MariadbOperatorImage, volumeMounts, envVar, resources, mariadb, securityContext)
}

func (b *Builder) jobMariadbContainer(cmd *cmd.Command, volumeMounts []corev1.VolumeMount, envVar []v1.EnvVar,
	resources *corev1.ResourceRequirements, mariadb *mariadbv1alpha1.MariaDB,
	securityContext *corev1.SecurityContext) (*corev1.Container, error) {

	return b.jobContainer("mariadb", cmd, mariadb.Spec.Image, volumeMounts, envVar, resources, mariadb, securityContext)
}

func jobBatchStorageVolume(volumeSource *corev1.VolumeSource, s3 *mariadbv1alpha1.S3) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes :=
		[]corev1.Volume{
			{
				Name:         batchStorageVolume,
				VolumeSource: *volumeSource,
			},
		}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      batchStorageVolume,
			MountPath: batchStorageMountPath,
		},
	}
	if s3 != nil && s3.TLS != nil && s3.TLS.Enabled && s3.TLS.CASecretKeyRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: batchS3PKI,
			VolumeSource: corev1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: s3.TLS.CASecretKeyRef.Name,
				},
			},
		})
		volumeMounts = append(volumeMounts, v1.VolumeMount{
			Name:      batchS3PKI,
			MountPath: batchS3PKIMountPath,
		})
	}
	return volumes, volumeMounts
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
				SecretKeyRef: &mariadb.Spec.RootPasswordSecretKeyRef.SecretKeySelector,
			},
		},
	}
}

func jobS3Env(s3 *mariadbv1alpha1.S3) []v1.EnvVar {
	if s3 == nil {
		return nil
	}
	env := []v1.EnvVar{
		{
			Name: batchS3AccessKeyId,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &s3.AccessKeyIdSecretKeyRef,
			},
		},
		{
			Name: batchS3SecretAccessKey,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &s3.SecretAccessKeySecretKeyRef,
			},
		},
	}
	if s3.SessionTokenSecretKeyRef != nil {
		env = append(env, v1.EnvVar{
			Name: batchS3SessionTokenKey,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: s3.SessionTokenSecretKeyRef,
			},
		})
	}
	return env
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
								Path: batchScriptsSqlFile,
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
