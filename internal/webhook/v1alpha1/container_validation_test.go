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
