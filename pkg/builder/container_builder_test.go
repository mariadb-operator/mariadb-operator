package builder

import (
	"os"
	"slices"
	"sort"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariadbStartupProbe", func() {
	DescribeTable("should build the expected probe",
		func(mariadb *mariadbv1alpha1.MariaDB, wantProbe *corev1.Probe) {
			probe, err := mariadbStartupProbe(mariadb)
			Expect(err).NotTo(HaveOccurred())
			Expect(probe).To(Equal(wantProbe))
		},
		Entry("MariaDB",
			&mariadbv1alpha1.MariaDB{},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB with thresholds",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							FailureThreshold: 10,
							TimeoutSeconds:   5,
							PeriodSeconds:    10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
				FailureThreshold:    10,
			},
		),
		Entry("MariaDB custom",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								Exec: &mariadbv1alpha1.ExecAction{
									Command: []string{
										"bash",
										"-c",
										"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
									},
								},
							},
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				FailureThreshold: 10,
				TimeoutSeconds:   10,
				PeriodSeconds:    10,
			},
		),
		Entry("MariaDB replication",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication with thresholds",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				FailureThreshold:    10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication custom",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/liveness-custom",
									Port: intstr.FromInt(5555),
								},
							},
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				FailureThreshold:    10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication with standalone probe",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							StandaloneProbes: ptr.To(true),
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB Galera",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB Galera with thresholds",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				FailureThreshold:    10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB Galera custom",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/liveness-custom",
									Port: intstr.FromInt(5555),
								},
							},
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				FailureThreshold:    10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
	)
})

var _ = Describe("MariadbLivenessProbe", func() {
	DescribeTable("should build the expected probe",
		func(mariadb *mariadbv1alpha1.MariaDB, wantProbe *corev1.Probe) {
			probe, err := mariadbLivenessProbe(mariadb)
			Expect(err).NotTo(HaveOccurred())
			Expect(probe).To(Equal(wantProbe))
		},
		Entry("MariaDB",
			&mariadbv1alpha1.MariaDB{},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB with thresholds",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB custom",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								Exec: &mariadbv1alpha1.ExecAction{
									Command: []string{
										"bash",
										"-c",
										"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
									},
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication with thresholds",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication custom",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/liveness-custom",
									Port: intstr.FromInt(5566),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication with standalone probe",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							StandaloneProbes: ptr.To(true),
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB Galera",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB Galera with thresholds",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB Galera custom",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/liveness-custom",
									Port: intstr.FromInt(5566),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
	)
})

var _ = Describe("MariadbReadinessProbe", func() {
	DescribeTable("should build the expected probe",
		func(mariadb *mariadbv1alpha1.MariaDB, wantProbe *corev1.Probe) {
			probe, err := mariadbReadinessProbe(mariadb)
			Expect(err).NotTo(HaveOccurred())
			Expect(probe).To(Equal(wantProbe))
		},
		Entry("MariaDB empty",
			&mariadbv1alpha1.MariaDB{},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB with thresholds",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB custom",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								Exec: &mariadbv1alpha1.ExecAction{
									Command: []string{
										"bash",
										"-c",
										"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
									},
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication with thresholds",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication custom",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/readiness-custom",
									Port: intstr.FromInt(5566),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB replication with ignored standalone probe",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							StandaloneProbes: ptr.To(true),
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB Galera",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB Galera with thresholds",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MariaDB Galera custom",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/readiness-custom",
									Port: intstr.FromInt(5566),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
	)
})

var _ = Describe("MaxScaleProbe", func() {
	DescribeTable("should build the expected probe",
		func(maxScale *mariadbv1alpha1.MaxScale, probe *mariadbv1alpha1.Probe, wantProbe *corev1.Probe) {
			got := maxscaleProbe(maxScale, probe)
			Expect(got).To(Equal(wantProbe))
		},
		Entry("MaxScale empty",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			&mariadbv1alpha1.Probe{},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		),
		Entry("MaxScale partial",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			&mariadbv1alpha1.Probe{
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MaxScale full",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			&mariadbv1alpha1.Probe{
				ProbeHandler: mariadbv1alpha1.ProbeHandler{
					TCPSocket: &mariadbv1alpha1.TCPSocketAction{
						Host: "custom",
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Host: "custom",
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
		Entry("MaxScale Probe with Failure Threshold",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			&mariadbv1alpha1.Probe{
				ProbeHandler: mariadbv1alpha1.ProbeHandler{
					HTTPGet: &mariadbv1alpha1.HTTPGetAction{
						Path: "/custom",
						Port: intstr.FromInt(8989),
					},
				},
				FailureThreshold:    10,
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
			&corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/custom",
						Port: intstr.FromInt(8989),
					},
				},
				FailureThreshold:    10,
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		),
	)
})

var _ = Describe("ContainerSecurityContext", func() {
	It("should build the expected security context", func() {
		builder := newDefaultTestBuilder()
		tpl := &mariadbv1alpha1.ContainerTemplate{}

		container, err := builder.buildContainerWithTemplate("mariadb:10.6", corev1.PullIfNotPresent, tpl)
		Expect(err).NotTo(HaveOccurred())
		Expect(container.SecurityContext).To(BeNil())

		tpl = &mariadbv1alpha1.ContainerTemplate{
			SecurityContext: &mariadbv1alpha1.SecurityContext{
				RunAsUser: ptr.To(mysqlUser),
			},
		}
		container, err = builder.buildContainerWithTemplate("mariadb:10.6", corev1.PullIfNotPresent, tpl)
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
		dsc, err := discovery.NewFakeDiscovery(resource)
		Expect(err).NotTo(HaveOccurred())
		builder = newTestBuilder(dsc)

		container, err = builder.buildContainerWithTemplate("mariadb:10.6", corev1.PullIfNotPresent, tpl)
		Expect(err).NotTo(HaveOccurred())
		Expect(container.SecurityContext).To(BeNil())
	})
})

var _ = Describe("BuildContainerLifecycle", func() {
	DescribeTable("should build the expected lifecycle",
		func(container *mariadbv1alpha1.ContainerTemplate, opts []mariadbPodOpt, want *corev1.Lifecycle) {
			builder := newDefaultTestBuilder()
			got, err := builder.buildContainerWithTemplate("mariadb", corev1.PullIfNotPresent, container, opts...)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Lifecycle).To(Equal(want))
		},
		Entry("no lifecycle",
			&mariadbv1alpha1.ContainerTemplate{},
			[]mariadbPodOpt{
				withLifecycle(true),
			},
			nil,
		),
		Entry("with postStart lifecycle",
			&mariadbv1alpha1.ContainerTemplate{
				Lifecycle: &mariadbv1alpha1.Lifecycle{
					PostStart: &mariadbv1alpha1.LifecycleHandler{
						Exec: &mariadbv1alpha1.ExecAction{
							Command: []string{"echo", "hello"},
						},
					},
				},
			},
			[]mariadbPodOpt{
				withLifecycle(true),
			},
			&corev1.Lifecycle{
				PostStart: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"echo", "hello"},
					},
				},
			},
		),
		Entry("with preStop lifecycle",
			&mariadbv1alpha1.ContainerTemplate{
				Lifecycle: &mariadbv1alpha1.Lifecycle{
					PreStop: &mariadbv1alpha1.LifecycleHandler{
						Exec: &mariadbv1alpha1.ExecAction{
							Command: []string{"echo", "hello"},
						},
					},
				},
			},
			[]mariadbPodOpt{
				withLifecycle(true),
			},
			&corev1.Lifecycle{
				PreStop: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"echo", "hello"},
					},
				},
			},
		),
		Entry("without lifecycle",
			&mariadbv1alpha1.ContainerTemplate{
				Lifecycle: &mariadbv1alpha1.Lifecycle{
					PreStop: &mariadbv1alpha1.LifecycleHandler{
						Exec: &mariadbv1alpha1.ExecAction{
							Command: []string{"echo", "hello"},
						},
					},
				},
			},
			[]mariadbPodOpt{
				withLifecycle(false),
			},
			nil,
		),
		Entry("defaults to no lifecycle",
			&mariadbv1alpha1.ContainerTemplate{
				Lifecycle: &mariadbv1alpha1.Lifecycle{
					PreStop: &mariadbv1alpha1.LifecycleHandler{
						Exec: &mariadbv1alpha1.ExecAction{
							Command: []string{"echo", "hello"},
						},
					},
				},
			},
			nil,
			nil,
		),
	)
})

var _ = Describe("MariadbEnv", func() {
	DescribeTable("should build the expected env",
		func(mariadb *mariadbv1alpha1.MariaDB, wantEnv []corev1.EnvVar, setClusterName bool) {
			if setClusterName {
				DeferCleanup(os.Setenv, "CLUSTER_NAME", os.Getenv("CLUSTER_NAME"))
				os.Setenv("CLUSTER_NAME", "example.com")
			}
			env, err := mariadbEnv(mariadb)
			Expect(err).NotTo(HaveOccurred())

			sortedWantEnv := sortEnvVars(wantEnv)
			sortedEnv := sortEnvVars(env)

			Expect(sortedEnv).To(Equal(sortedWantEnv))
		},
		Entry("MariaDB empty",
			&mariadbv1alpha1.MariaDB{},
			defaultEnv(nil),
			false,
		),
		Entry("MariaDB cluster name",
			&mariadbv1alpha1.MariaDB{},
			defaultEnv([]corev1.EnvVar{
				{
					Name:  "CLUSTER_NAME",
					Value: "example.com",
				},
			}),
			true,
		),
		Entry("MariaDB tcp port",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 12345,
				},
			},
			defaultEnv([]corev1.EnvVar{
				{
					Name:  "MYSQL_TCP_PORT",
					Value: strconv.Itoa(12345),
				},
			}),
			false,
		),
		Entry("MariaDB name",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example",
				},
			},
			defaultEnv([]corev1.EnvVar{
				{
					Name:  "MARIADB_NAME",
					Value: "example",
				},
			}),
			false,
		),
		Entry("MariaDB root empty password",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootEmptyPassword: ptr.To(true),
				},
			},
			defaultEnv([]corev1.EnvVar{
				{
					Name:  "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD",
					Value: "yes",
				},
			}),
			false,
		),
		Entry("MariaDB timeZone",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TimeZone: ptr.To("UTC"),
				},
			},
			removeEnv(defaultEnv(nil), "MYSQL_INITDB_SKIP_TZINFO"),
			false,
		),
		Entry("MariaDB TLS",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			append(defaultEnv(nil),
				[]corev1.EnvVar{
					{
						Name:  "TLS_ENABLED",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "TLS_CA_CERT_PATH",
						Value: builderpki.CACertPath,
					},
					{
						Name:  "TLS_SERVER_CERT_PATH",
						Value: builderpki.ServerCertPath,
					},
					{
						Name:  "TLS_SERVER_KEY_PATH",
						Value: builderpki.ServerKeyPath,
					},
					{
						Name:  "TLS_CLIENT_CERT_PATH",
						Value: builderpki.ClientCertPath,
					},
					{
						Name:  "TLS_CLIENT_KEY_PATH",
						Value: builderpki.ClientKeyPath,
					},
				}...),
			false,
		),
		Entry("MariaDB replication",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-repl",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							GtidStrictMode:     ptr.To(true),
							GtidDomainID:       ptr.To(10),
							ServerIDStartIndex: ptr.To(100),
							SemiSyncEnabled:    ptr.To(true),
							SemiSyncAckTimeout: &metav1.Duration{Duration: 10 * time.Second},
							SemiSyncWaitPoint:  ptr.To(mariadbv1alpha1.WaitPointAfterCommit),
						},
					},
				},
			},
			append(
				defaultEnv([]corev1.EnvVar{
					{
						Name:  "MARIADB_NAME",
						Value: "mariadb-repl",
					},
				}),
				[]corev1.EnvVar{
					{
						Name:  "MARIADB_REPL_ENABLED",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "MARIADB_REPL_GTID_STRICT_MODE",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "MARIADB_REPL_GTID_DOMAIN_ID",
						Value: "10",
					},
					{
						Name:  "MARIADB_REPL_SERVER_ID_START_INDEX",
						Value: "100",
					},
					{
						Name:  "MARIADB_REPL_SEMI_SYNC_ENABLED",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "MARIADB_REPL_SEMI_SYNC_MASTER_TIMEOUT",
						Value: "10000",
					},
					{
						Name:  "MARIADB_REPL_SEMI_SYNC_MASTER_WAIT_POINT",
						Value: "AFTER_COMMIT",
					},
				}...),
			false,
		),
		Entry("MariaDB Galera TLS",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-galera",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			append(
				defaultEnv([]corev1.EnvVar{
					{
						Name:  "MARIADB_NAME",
						Value: "mariadb-galera",
					},
				}),
				[]corev1.EnvVar{
					{
						Name:  "TLS_ENABLED",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "TLS_CA_CERT_PATH",
						Value: builderpki.CACertPath,
					},
					{
						Name:  "TLS_SERVER_CERT_PATH",
						Value: builderpki.ServerCertPath,
					},
					{
						Name:  "TLS_SERVER_KEY_PATH",
						Value: builderpki.ServerKeyPath,
					},
					{
						Name:  "TLS_CLIENT_CERT_PATH",
						Value: builderpki.ClientCertPath,
					},
					{
						Name:  "TLS_CLIENT_KEY_PATH",
						Value: builderpki.ClientKeyPath,
					},
					{
						Name:  "WSREP_SST_OPT_REMOTE_AUTH",
						Value: "mariadb-galera-client:",
					},
				}...),
			false,
		),
		Entry("MariaDB env append",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Env: []mariadbv1alpha1.EnvVar{
							{
								Name:  "FOO_BAR_BAZ",
								Value: "LOREM_IPSUM_DOLOR",
							},
						},
					},
				},
			},
			append(defaultEnv(nil), corev1.EnvVar{
				Name:  "FOO_BAR_BAZ",
				Value: "LOREM_IPSUM_DOLOR",
			}),
			false,
		),
		Entry("MariaDB env override",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Env: []mariadbv1alpha1.EnvVar{
							{
								Name:  "MYSQL_TCP_PORT",
								Value: strconv.Itoa(12345),
							},
							{
								Name:  "MARIADB_ROOT_HOST",
								Value: "1.2.3.4",
							},
							{
								Name:  "CLUSTER_NAME",
								Value: "foo.bar",
							},
							{
								Name:  "POD_NAME",
								Value: "foo",
							},
							{
								Name:  "POD_NAMESPACE",
								Value: "foo",
							},
							{
								Name:  "POD_IP",
								Value: "1.2.3.4",
							},
							{
								Name:  "MARIADB_NAME",
								Value: "foo",
							},
							{
								Name:  "MARIADB_ROOT_PASSWORD",
								Value: "foo",
							},
							{
								Name:  "MYSQL_INITDB_SKIP_TZINFO",
								Value: "0",
							},
						},
					},
				},
			},
			[]corev1.EnvVar{
				{
					Name:  "MYSQL_TCP_PORT",
					Value: strconv.Itoa(12345),
				},
				{
					Name:  "MARIADB_ROOT_HOST",
					Value: "1.2.3.4",
				},
				{
					Name:  "CLUSTER_NAME",
					Value: "foo.bar",
				},
				{
					Name:  "POD_NAME",
					Value: "foo",
				},
				{
					Name:  "POD_NAMESPACE",
					Value: "foo",
				},
				{
					Name:  "POD_IP",
					Value: "1.2.3.4",
				},
				{
					Name:  "MARIADB_NAME",
					Value: "foo",
				},
				{
					Name:  "MARIADB_ROOT_PASSWORD",
					Value: "foo",
				},
				{
					Name:  "MYSQL_INITDB_SKIP_TZINFO",
					Value: "0",
				},
			},
			false,
		),
	)
})

var _ = Describe("S3Env", func() {
	DescribeTable("should build the expected env",
		func(s3 *mariadbv1alpha1.S3, expectedEnv []string) {
			env := s3Env(s3)

			if expectedEnv == nil {
				Expect(env).To(BeNil())
				return
			}

			Expect(env).To(HaveLen(len(expectedEnv)))

			for _, expectedName := range expectedEnv {
				found := slices.ContainsFunc(env, func(e corev1.EnvVar) bool {
					return e.Name == expectedName
				})
				Expect(found).To(BeTrue())
			}
		},
		Entry("nil S3",
			nil,
			nil,
		),
		Entry("S3 with access key only",
			&mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
			},
			[]string{S3AccessKeyId, S3SecretAccessKey},
		),
		Entry("S3 with session token",
			&mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
				SessionTokenSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "session-token",
				},
			},
			[]string{S3AccessKeyId, S3SecretAccessKey, S3SessionTokenKey},
		),
		Entry("S3 with SSE-C",
			&mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
				SSEC: &mariadbv1alpha1.SSECConfig{
					CustomerKeySecretKeyRef: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: "ssec-key",
						},
						Key: "customer-key",
					},
				},
			},
			[]string{S3AccessKeyId, S3SecretAccessKey, S3SSECCustomerKey},
		),
		Entry("S3 with all options",
			&mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
				SessionTokenSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "session-token",
				},
				SSEC: &mariadbv1alpha1.SSECConfig{
					CustomerKeySecretKeyRef: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: "ssec-key",
						},
						Key: "customer-key",
					},
				},
			},
			[]string{S3AccessKeyId, S3SecretAccessKey, S3SessionTokenKey, S3SSECCustomerKey},
		),
	)
})

var _ = Describe("ContainerArgs", func() {
	DescribeTable("should build the expected args",
		func(mariadb *mariadbv1alpha1.MariaDB, wantArgs []string) {
			args := mariadbArgs(mariadb)
			Expect(args).To(Equal(wantArgs))
		},
		Entry("MariaDB args empty",
			&mariadbv1alpha1.MariaDB{},
			nil,
		),
		Entry("MariaDB args verbose",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Args: []string{"--verbose"},
					},
				},
			},
			[]string{
				"--verbose",
			},
		),
		Entry("MariaDB args verbose",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-test",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Args: []string{"--verbose"},
					},
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			[]string{
				"--verbose",
			},
		),
	)
})

var _ = Describe("MariadbContainers", func() {
	DescribeTable("should build the expected container",
		func(mariadb *mariadbv1alpha1.MariaDB, wantName string, wantEnvKeys []string, wantVolumeMountKeys []string) {
			builder := newDefaultTestBuilder()
			containers, err := builder.mariadbContainers(mariadb)
			Expect(err).NotTo(HaveOccurred())

			container := containers[1]

			Expect(container.Name).To(Equal(wantName))
			if wantEnvKeys != nil {
				idx := datastructures.NewIndex(container.Env, func(env corev1.EnvVar) string {
					return env.Name
				})
				Expect(datastructures.AllExists(idx, wantEnvKeys...)).To(BeTrue())
			}
			if wantVolumeMountKeys != nil {
				idx := datastructures.NewIndex(container.VolumeMounts, func(volumeMount corev1.VolumeMount) string {
					return volumeMount.Name
				})
				Expect(datastructures.AllExists(idx, wantVolumeMountKeys...)).To(BeTrue())
			}
		},
		Entry("Without sidecar container name",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						SidecarContainers: []mariadbv1alpha1.Container{
							{
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep infinity",
								},
							},
						},
					},
				},
			},
			"sidecar-0",
			nil,
			nil,
		),
		Entry("With sidecar container name",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						SidecarContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep infinity",
								},
							},
						},
					},
				},
			},
			"busybox",
			nil,
			nil,
		),
		Entry("With env",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						SidecarContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
								Env: []mariadbv1alpha1.EnvVar{
									{
										Name:  "TEST",
										Value: "TEST",
									},
									{
										Name:  "FOO",
										Value: "FOO",
									},
									{
										Name:  "BAR",
										Value: "BAR",
									},
								},
							},
						},
					},
				},
			},
			"busybox",
			[]string{"TEST", "FOO", "BAR"},
			nil,
		),
		Entry("With volumeMount",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						SidecarContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
								VolumeMounts: []mariadbv1alpha1.VolumeMount{
									{
										Name:      "TEST",
										MountPath: "/test",
									},
									{
										Name:      "FOO",
										MountPath: "/foo",
									},
									{
										Name:      "BAR",
										MountPath: "/bar",
									},
								},
							},
						},
					},
				},
			},
			"busybox",
			nil,
			[]string{"TEST", "FOO", "BAR"},
		),
	)
})

var _ = Describe("MariadbInitContainers", func() {
	DescribeTable("should build the expected init container",
		func(mariadb *mariadbv1alpha1.MariaDB, wantName string, wantEnvKeys []string, wantVolumeMountKeys []string) {
			builder := newDefaultTestBuilder()
			initContainers, err := builder.mariadbInitContainers(mariadb)
			Expect(err).NotTo(HaveOccurred())

			initContainer := initContainers[0]

			Expect(initContainer.Name).To(Equal(wantName))
			if wantEnvKeys != nil {
				idx := datastructures.NewIndex(initContainer.Env, func(env corev1.EnvVar) string {
					return env.Name
				})
				Expect(datastructures.AllExists(idx, wantEnvKeys...)).To(BeTrue())
			}
			if wantVolumeMountKeys != nil {
				idx := datastructures.NewIndex(initContainer.VolumeMounts, func(volumeMount corev1.VolumeMount) string {
					return volumeMount.Name
				})
				Expect(datastructures.AllExists(idx, wantVolumeMountKeys...)).To(BeTrue())
			}
		},
		Entry("Without container name",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						InitContainers: []mariadbv1alpha1.Container{
							{
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
							},
						},
					},
				},
			},
			"init-0",
			nil,
			nil,
		),
		Entry("With container name",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						InitContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
							},
						},
					},
				},
			},
			"busybox",
			nil,
			nil,
		),
		Entry("With env",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						InitContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
								Env: []mariadbv1alpha1.EnvVar{
									{
										Name:  "TEST",
										Value: "TEST",
									},
									{
										Name:  "FOO",
										Value: "FOO",
									},
									{
										Name:  "BAR",
										Value: "BAR",
									},
								},
							},
						},
					},
				},
			},
			"busybox",
			[]string{"TEST", "FOO", "BAR"},
			nil,
		),
		Entry("With volumeMount",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						InitContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
								VolumeMounts: []mariadbv1alpha1.VolumeMount{
									{
										Name:      "TEST",
										MountPath: "/test",
									},
									{
										Name:      "FOO",
										MountPath: "/foo",
									},
									{
										Name:      "BAR",
										MountPath: "/bar",
									},
								},
							},
						},
					},
				},
			},
			"busybox",
			nil,
			[]string{"TEST", "FOO", "BAR"},
		),
	)
})

var _ = Describe("MaxscaleContainers", func() {
	DescribeTable("should build the expected container",
		func(maxscale *mariadbv1alpha1.MaxScale, wantCommand []string, wantArgs []string) {
			builder := newDefaultTestBuilder()
			containers, err := builder.maxscaleContainers(maxscale)
			Expect(err).NotTo(HaveOccurred())

			container := containers[0]

			Expect(container.Command[0]).To(Equal(wantCommand[0]))
			Expect(container.Args).To(Equal(wantArgs))
		},
		Entry("Without custom command and args",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{},
				},
			},
			[]string{"maxscale"},
			[]string{"--config", "/etc/config/maxscale.cnf", "-dU", "maxscale", "-l", "stdout"},
		),
		Entry("With custom command",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Command: []string{"maxscale-test"},
					},
				},
			},
			[]string{"maxscale-test"},
			[]string{"--config", "/etc/config/maxscale.cnf", "-dU", "maxscale", "-l", "stdout"},
		),
		Entry("With custom command and args",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Command: []string{"maxscale-test"},
						Args:    []string{"--test", "--unit"},
					},
				},
			},
			[]string{"maxscale-test"},
			[]string{"--test", "--unit"},
		),
	)
})

var _ = Describe("MariadbStorageVolumeMount", func() {
	DescribeTable("should build the expected volume mount",
		func(mariadb *mariadbv1alpha1.MariaDB, wantSubPath string) {
			vm := mariadbStorageVolumeMount(mariadb)
			Expect(vm.SubPath).To(Equal(wantSubPath))
		},
		Entry("no galera configured",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{}},
			"",
		),
		Entry("galera enabled reuse disabled",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Config: mariadbv1alpha1.GaleraConfig{
								ReuseStorageVolume: ptr.To(false),
							},
						},
					},
				},
			},
			"",
		),
		Entry("galera enabled reuse enabled",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Config: mariadbv1alpha1.GaleraConfig{
								ReuseStorageVolume: ptr.To(true),
							},
						},
					},
				},
			},
			StorageVolume,
		),
	)
})

func defaultEnv(overrides []corev1.EnvVar) []corev1.EnvVar {
	mysqlTcpPort := corev1.EnvVar{
		Name:  "MYSQL_TCP_PORT",
		Value: strconv.Itoa(0),
	}
	clusterName := corev1.EnvVar{
		Name:  "CLUSTER_NAME",
		Value: "cluster.local",
	}
	mariadbName := corev1.EnvVar{
		Name:  "MARIADB_NAME",
		Value: "",
	}
	mariadbRootPassword := corev1.EnvVar{
		Name: "MARIADB_ROOT_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{},
		},
	}
	mysqlInitdbSkipTzinfo := corev1.EnvVar{
		Name:  "MYSQL_INITDB_SKIP_TZINFO",
		Value: "1",
	}
	defaults := map[string]corev1.EnvVar{
		mysqlTcpPort.Name:          mysqlTcpPort,
		clusterName.Name:           clusterName,
		mariadbName.Name:           mariadbName,
		mariadbRootPassword.Name:   mariadbRootPassword,
		mysqlInitdbSkipTzinfo.Name: mysqlInitdbSkipTzinfo,
	}
	for _, override := range overrides {
		if _, ok := defaults[override.Name]; ok {
			defaults[override.Name] = override
		}
		if override.Name == "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD" {
			defaults[mariadbRootPassword.Name] = override
		}
	}

	return []corev1.EnvVar{
		defaults[mysqlTcpPort.Name],
		{
			Name:  "MARIADB_ROOT_HOST",
			Value: "%",
		},
		defaults[clusterName.Name],
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		defaults[mariadbName.Name],
		defaults[mariadbRootPassword.Name],
		defaults[mysqlInitdbSkipTzinfo.Name],
	}
}

func removeEnv(env []corev1.EnvVar, key string) []corev1.EnvVar {
	var result []corev1.EnvVar
	for _, e := range env {
		if e.Name != key {
			result = append(result, e)
		}
	}
	return result
}

func sortEnvVars(env []corev1.EnvVar) []corev1.EnvVar {
	sortedEnv := make([]corev1.EnvVar, len(env))
	copy(sortedEnv, env)
	sort.SliceStable(sortedEnv, func(i, j int) bool {
		return sortedEnv[i].Name < sortedEnv[j].Name
	})
	return sortedEnv
}
