package builder

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/pki"
	cmd "github.com/mariadb-operator/mariadb-operator/v25/pkg/command"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/interfaces"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pki"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func (b *Builder) jobContainer(name string, cmd *cmd.Command, image string, volumeMounts []corev1.VolumeMount, env []corev1.EnvVar,
	resources *corev1.ResourceRequirements, mariadb interfaces.Imager,
	securityContext *mariadbv1alpha1.SecurityContext) (*corev1.Container, error) {
	sc, err := b.buildContainerSecurityContext(securityContext)
	if err != nil {
		return nil, err
	}

	container := corev1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: mariadb.GetImagePullPolicy(),
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

func (b *Builder) jobMariadbOperatorContainer(cmd *cmd.Command, volumeMounts []corev1.VolumeMount, envVar []corev1.EnvVar,
	resources *corev1.ResourceRequirements, mariadb interfaces.Imager, env *environment.OperatorEnv,
	securityContext *mariadbv1alpha1.SecurityContext) (*corev1.Container, error) {

	return b.jobContainer("mariadb-operator", cmd, env.MariadbOperatorImage, volumeMounts, envVar, resources, mariadb, securityContext)
}

func (b *Builder) jobMariadbContainer(cmd *cmd.Command, env *environment.OperatorEnv, volumeMounts []corev1.VolumeMount,
	envVar []corev1.EnvVar, resources *corev1.ResourceRequirements, mariadb interfaces.Imager,
	securityContext *mariadbv1alpha1.SecurityContext) (*corev1.Container, error) {

	return b.jobContainer("mariadb", cmd, mariadb.GetImage(env), volumeMounts, envVar, resources, mariadb, securityContext)
}

func jobBatchStorageVolume(storageVolume mariadbv1alpha1.StorageVolumeSource,
	s3 *mariadbv1alpha1.S3, mariadb interfaces.TLSProvider) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes :=
		[]corev1.Volume{
			{
				Name:         batchStorageVolume,
				VolumeSource: storageVolume.ToKubernetesType(),
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
				Secret: &corev1.SecretVolumeSource{
					SecretName: s3.TLS.CASecretKeyRef.Name,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      batchS3PKI,
			MountPath: batchS3PKIMountPath,
		})
	}
	if mariadb.IsTLSEnabled() {
		tlsVolumes, tlsVolumeMounts := mariadbTLSVolumes(mariadb)
		volumes = append(volumes, tlsVolumes...)
		volumeMounts = append(volumeMounts, tlsVolumeMounts...)
	}
	return volumes, volumeMounts
}

func jobPhysicalBackupVolumes(storageVolume mariadbv1alpha1.StorageVolumeSource,
	s3 *mariadbv1alpha1.S3, mariadb *mariadbv1alpha1.MariaDB, podIndex *int) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes, volumeMounts := jobBatchStorageVolume(storageVolume, s3, mariadb)

	volumes = append(volumes, corev1.Volume{
		Name: StorageVolume,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: mariadb.PVCKey(StorageVolume, *podIndex).Name,
			},
		},
	})
	volumeMounts = append(volumeMounts, mariadbStorageVolumeMount(mariadb))

	return volumes, volumeMounts
}

func jobEnv(mariadb interfaces.Connector) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  batchUserEnv,
			Value: mariadb.GetSUName(),
		},
	}

	suCredential := mariadb.GetSUCredential()
	if suCredential != nil {
		env = append(env, corev1.EnvVar{
			Name: batchPasswordEnv,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(suCredential.ToKubernetesType()),
			},
		})
	}

	return env
}

func jobS3Env(s3 *mariadbv1alpha1.S3) []corev1.EnvVar {
	if s3 == nil {
		return nil
	}
	var env []corev1.EnvVar
	if s3.AccessKeyIdSecretKeyRef != nil {
		env = append(env, corev1.EnvVar{
			Name: batchS3AccessKeyId,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(s3.AccessKeyIdSecretKeyRef.ToKubernetesType()),
			},
		})
	}
	if s3.AccessKeyIdSecretKeyRef != nil {
		env = append(env, corev1.EnvVar{
			Name: batchS3SecretAccessKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(s3.SecretAccessKeySecretKeyRef.ToKubernetesType()),
			},
		})
	}
	if s3.SessionTokenSecretKeyRef != nil {
		env = append(env, corev1.EnvVar{
			Name: batchS3SessionTokenKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(s3.SessionTokenSecretKeyRef.ToKubernetesType()),
			},
		})
	}
	if s3.SSEC != nil {
		env = append(env, corev1.EnvVar{
			Name: batchS3SSECCustomerKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(s3.SSEC.CustomerKeySecretKeyRef.ToKubernetesType()),
			},
		})
	}
	return env
}

func jobResources(resources *mariadbv1alpha1.ResourceRequirements) *corev1.ResourceRequirements {
	if resources != nil {
		return ptr.To(resources.ToKubernetesType())
	}
	return nil
}

func sqlJobvolumes(sqlJob *mariadbv1alpha1.SqlJob, mariadb interfaces.TLSProvider) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{
		{
			Name: batchScriptsVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: sqlJob.Spec.SqlConfigMapKeyRef.LocalObjectReference.ToKubernetesType(),
					Items: []corev1.KeyToPath{
						{
							Key:  sqlJob.Spec.SqlConfigMapKeyRef.Key,
							Path: batchScriptsSqlFile,
						},
					},
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      batchScriptsVolume,
			MountPath: batchScriptsMountPath,
		},
	}

	if sqlJob.Spec.TLSCACertSecretRef != nil && sqlJob.Spec.TLSClientCertSecretRef != nil {
		volumes = append(volumes, []corev1.Volume{
			{
				Name: builderpki.PKIVolume,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: sqlJob.Spec.TLSCACertSecretRef.Name,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  pki.CACertKey,
											Path: pki.CACertKey,
										},
									},
								},
							},
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: sqlJob.Spec.TLSClientCertSecretRef.Name,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  pki.TLSCertKey,
											Path: builderpki.ClientCertKey,
										},
										{
											Key:  pki.TLSKeyKey,
											Path: builderpki.ClientKeyKey,
										},
									},
								},
							},
						},
					},
				},
			},
		}...)
		volumeMounts = append(volumeMounts, []corev1.VolumeMount{
			{
				Name:      builderpki.PKIVolume,
				MountPath: builderpki.PKIMountPath,
			},
		}...)
	} else if mariadb.IsTLSEnabled() {
		tlsVolumes, tlsVolumeMounts := mariadbTLSVolumes(mariadb)
		volumes = append(volumes, tlsVolumes...)
		volumeMounts = append(volumeMounts, tlsVolumeMounts...)
	}

	return volumes, volumeMounts
}

func sqlJobEnv(sqlJob *mariadbv1alpha1.SqlJob) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  batchUserEnv,
			Value: sqlJob.Spec.Username,
		},
		{
			Name: batchPasswordEnv,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(sqlJob.Spec.PasswordSecretKeyRef.ToKubernetesType()),
			},
		},
	}
}
