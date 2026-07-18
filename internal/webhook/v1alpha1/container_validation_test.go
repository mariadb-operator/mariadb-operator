package v1alpha1

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Custom container inheritance validation", func() {
	It("preserves omitted Legacy behavior", func() {
		mariadb := &mariadbv1alpha1.MariaDB{}
		mariadb.Spec.InitContainers = []mariadbv1alpha1.Container{{Image: "busybox:1.36"}}
		mariadb.Spec.SidecarContainers = []mariadbv1alpha1.Container{{
			Image:       "busybox:1.36",
			Inheritance: &mariadbv1alpha1.ContainerInheritance{},
		}}
		Expect(validateContainers(mariadb)).To(Succeed())
	})

	It("validates init and sidecar policies independently", func() {
		mariadb := &mariadbv1alpha1.MariaDB{}
		mariadb.Spec.InitContainers = []mariadbv1alpha1.Container{{
			Image: "busybox:1.36",
			Inheritance: &mariadbv1alpha1.ContainerInheritance{
				Policy: mariadbv1alpha1.ContainerInheritanceIsolated,
			},
		}}
		mariadb.Spec.SidecarContainers = []mariadbv1alpha1.Container{{
			Image: "busybox:1.36",
			Inheritance: &mariadbv1alpha1.ContainerInheritance{
				Policy: mariadbv1alpha1.ContainerInheritanceSelected,
				Env:    []mariadbv1alpha1.ContainerEnvGroup{mariadbv1alpha1.ContainerEnvGroupRuntime},
			},
		}}
		Expect(validateContainers(mariadb)).To(Succeed())
	})

	It("treats omitted TLS as enabled by the API default", func() {
		mariadb := &mariadbv1alpha1.MariaDB{}
		mariadb.Spec.SidecarContainers = []mariadbv1alpha1.Container{{
			Image: "busybox:1.36",
			Inheritance: &mariadbv1alpha1.ContainerInheritance{
				Policy: mariadbv1alpha1.ContainerInheritanceSelected,
				Env:    []mariadbv1alpha1.ContainerEnvGroup{mariadbv1alpha1.ContainerEnvGroupTLS},
			},
		}}
		Expect(validateContainers(mariadb)).To(Succeed())
	})

	DescribeTable(
		"matches AgentAuth availability to usable agent basic-auth material",
		func(agent mariadbv1alpha1.Agent, shouldSucceed bool) {
			mariadb := &mariadbv1alpha1.MariaDB{}
			mariadb.Spec.Replication = &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Agent: agent,
				},
			}
			mariadb.Spec.SidecarContainers = []mariadbv1alpha1.Container{{
				Image: "busybox:1.36",
				Inheritance: &mariadbv1alpha1.ContainerInheritance{
					Policy: mariadbv1alpha1.ContainerInheritanceSelected,
					VolumeMounts: []mariadbv1alpha1.ContainerVolumeMountGroup{
						mariadbv1alpha1.ContainerVolumeMountGroupAgentAuth,
					},
				},
			}}

			err := validateContainers(mariadb)
			if shouldSucceed {
				Expect(err).ToNot(HaveOccurred())
				return
			}
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.sidecarContainers[0].inheritance.volumeMounts[0]"))
		},
		Entry(
			"accepts enabled basic auth with a password Secret reference",
			mariadbv1alpha1.Agent{BasicAuth: &mariadbv1alpha1.BasicAuth{
				Enabled: true,
				PasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{Name: "agent-auth"},
						Key:                  "password",
					},
				},
			}},
			true,
		),
		Entry(
			"rejects Kubernetes auth only",
			mariadbv1alpha1.Agent{KubernetesAuth: &mariadbv1alpha1.KubernetesAuth{Enabled: true}},
			false,
		),
		Entry(
			"rejects basic auth without a password Secret reference",
			mariadbv1alpha1.Agent{BasicAuth: &mariadbv1alpha1.BasicAuth{Enabled: true}},
			false,
		),
	)

	DescribeTable(
		"rejects invalid selections",
		func(mariadb *mariadbv1alpha1.MariaDB, expectedPath string) {
			err := validateContainers(mariadb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedPath))
		},
		Entry(
			"groups with Legacy",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{
				MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
					InitContainers: []mariadbv1alpha1.Container{{
						Image: "busybox:1.36",
						Inheritance: &mariadbv1alpha1.ContainerInheritance{
							Policy: mariadbv1alpha1.ContainerInheritanceLegacy,
							Env:    []mariadbv1alpha1.ContainerEnvGroup{mariadbv1alpha1.ContainerEnvGroupRuntime},
						},
					}},
				},
			}},
			"spec.initContainers[0].inheritance",
		),
		Entry(
			"unavailable replication group",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{
				MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
					SidecarContainers: []mariadbv1alpha1.Container{{
						Image: "busybox:1.36",
						Inheritance: &mariadbv1alpha1.ContainerInheritance{
							Policy: mariadbv1alpha1.ContainerInheritanceSelected,
							Env:    []mariadbv1alpha1.ContainerEnvGroup{mariadbv1alpha1.ContainerEnvGroupReplication},
						},
					}},
				},
			}},
			"spec.sidecarContainers[0].inheritance.env[0]",
		),
		Entry(
			"duplicate isolated mount path",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{
				MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
					InitContainers: []mariadbv1alpha1.Container{{
						Image: "busybox:1.36",
						Inheritance: &mariadbv1alpha1.ContainerInheritance{
							Policy: mariadbv1alpha1.ContainerInheritanceIsolated,
						},
						VolumeMounts: []mariadbv1alpha1.VolumeMount{
							{Name: "first", MountPath: "/data"},
							{Name: "second", MountPath: "/data"},
						},
					}},
				},
			}},
			"spec.initContainers[0].volumeMounts[1].mountPath",
		),
	)
})
