package builder

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	cmd "github.com/mariadb-operator/mariadb-operator/v26/pkg/command"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/interfaces"
	kadapter "github.com/mariadb-operator/mariadb-operator/v26/pkg/kubernetes/adapter"
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

	return b.jobMariadbContainerWithName("mariadb", cmd, env, volumeMounts, envVar, resources, mariadb, securityContext)
}

func (b *Builder) jobMariadbContainerWithName(name string, cmd *cmd.Command, env *environment.OperatorEnv,
	volumeMounts []corev1.VolumeMount, envVar []corev1.EnvVar, resources *corev1.ResourceRequirements, mariadb interfaces.Imager,
	securityContext *mariadbv1alpha1.SecurityContext) (*corev1.Container, error) {

	return b.jobContainer(name, cmd, mariadb.GetImage(env), volumeMounts, envVar, resources, mariadb, securityContext)
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

func physicalBackupJobEnv(mariadb *mariadbv1alpha1.MariaDB) []corev1.EnvVar {
	env := jobEnv(mariadb)

	if mariadb.Spec.Env != nil {
		env = append(env, kadapter.ToKubernetesSlice(mariadb.Spec.Env)...)
	}

	return env
}

func jobResources(resources *mariadbv1alpha1.ResourceRequirements) *corev1.ResourceRequirements {
	if resources != nil {
		return ptr.To(resources.ToKubernetesType())
	}
	return nil
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
