package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/command"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("JobContainerSecurityContext", func() {
	It("should build the container security context", func() {
		builder := newDefaultTestBuilder()
		cmd := command.NewCommand([]string{"mariadbd"}, []string{})
		image := "mariadb:10.6"
		volumeMounts := []corev1.VolumeMount{}
		envVar := []corev1.EnvVar{}
		resources := &corev1.ResourceRequirements{}
		mariadb := &mariadbv1alpha1.MariaDB{}
		var securityContext *mariadbv1alpha1.SecurityContext

		container, err := builder.jobContainer("mariadb", cmd, image, volumeMounts, envVar, resources, mariadb, securityContext)
		Expect(err).NotTo(HaveOccurred())
		Expect(container.SecurityContext).To(BeNil())

		securityContext = &mariadbv1alpha1.SecurityContext{
			RunAsUser: ptr.To(mysqlUser),
		}
		container, err = builder.jobContainer("mariadb", cmd, image, volumeMounts, envVar, resources, mariadb, securityContext)
		Expect(err).NotTo(HaveOccurred())
		Expect(container.SecurityContext).NotTo(BeNil())

		resource := &metav1.APIResourceList{
			GroupVersion: "security.openshift.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name: "securitycontextconstraints",
				},
			},
		}
		fakeDiscovery, err := discovery.NewFakeDiscovery(resource)
		Expect(err).NotTo(HaveOccurred())
		builder = newTestBuilder(fakeDiscovery)

		container, err = builder.jobContainer("mariadb", cmd, image, volumeMounts, envVar, resources, mariadb, securityContext)
		Expect(err).NotTo(HaveOccurred())
		Expect(container.SecurityContext).To(BeNil())
	})
})

var _ = Describe("PhysicalBackupJobEnv", func() {
	secretKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: "test",
		},
		Key: "test",
	}

	DescribeTable("building the env",
		func(mariadb *mariadbv1alpha1.MariaDB, expected []corev1.EnvVar) {
			got := physicalBackupJobEnv(mariadb)
			Expect(got).To(Equal(expected))
		},
		Entry("Environment with credentials",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: secretKeySelector,
						Generate:          false,
					},
				},
			},
			[]corev1.EnvVar{
				{Name: batchUserEnv, Value: "root"},
				{
					Name: batchPasswordEnv,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: ptr.To(secretKeySelector.ToKubernetesType()),
					},
				},
			},
		),
		Entry("Environment with credentials and additional env",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: secretKeySelector,
						Generate:          false,
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Env: []mariadbv1alpha1.EnvVar{
							{
								Name:  "TEST",
								Value: "TEST",
							},
						},
					},
				},
			},
			[]corev1.EnvVar{
				{Name: batchUserEnv, Value: "root"},
				{
					Name: batchPasswordEnv,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: ptr.To(secretKeySelector.ToKubernetesType()),
					},
				},
				{Name: "TEST", Value: "TEST"},
			},
		),
	)
})
