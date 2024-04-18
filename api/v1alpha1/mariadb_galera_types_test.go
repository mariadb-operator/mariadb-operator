package v1alpha1

import (
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariaDB Galera types", func() {
	env := &environment.OperatorEnv{
		MariadbGaleraInitImage:  "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
		MariadbGaleraAgentImage: "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
		MariadbGaleraLibPath:    "/usr/lib/galera/libgalera_smm.so",
	}
	Context("When creating a MariaDB Galera object", func() {
		mdbObjMeta := metav1.ObjectMeta{
			Name:      "mdb-galera-obj",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should default",
			func(mdb *MariaDB, galera, expected *Galera, env *environment.OperatorEnv) {
				galera.SetDefaults(mdb, env)
				Expect(galera).To(BeEquivalentTo(expected))
			},
			Entry(
				"Full default",
				&MariaDB{
					ObjectMeta: mdbObjMeta,
					Spec: MariaDBSpec{
						Replicas: 3,
					},
				},
				&Galera{
					Enabled: true,
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						SST:            SSTMariaBackup,
						ReplicaThreads: 1,
						InitContainer: Container{
							Image:           "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						AvailableWhenDonor: ptr.To(false),
						GaleraLibPath:      "/usr/lib/galera/libgalera_smm.so",
						Config: GaleraConfig{
							ReuseStorageVolume: ptr.To(false),
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
								},
							},
						},
						Primary: PrimaryGalera{
							PodIndex:          ptr.To(0),
							AutomaticFailover: ptr.To(true),
						},
						Agent: GaleraAgent{
							Image:           "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Port:            5555,
							KubernetesAuth: &KubernetesAuth{
								Enabled: true,
							},
							GracefulShutdownTimeout: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
						},
						Recovery: &GaleraRecovery{
							Enabled:                 true,
							MinClusterSize:          ptr.To(intstr.FromString("50%")),
							ClusterMonitorInterval:  ptr.To(metav1.Duration{Duration: 10 * time.Second}),
							ClusterHealthyTimeout:   ptr.To(metav1.Duration{Duration: 30 * time.Second}),
							ClusterBootstrapTimeout: ptr.To(metav1.Duration{Duration: 10 * time.Minute}),
							PodRecoveryTimeout:      ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
							PodSyncTimeout:          ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
						},
					},
				},
				env,
			),
			Entry(
				"Partial default",
				&MariaDB{
					ObjectMeta: mdbObjMeta,
					Spec: MariaDBSpec{
						Replicas: 3,
					},
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						SST:           SSTRsync,
						GaleraLibPath: "/usr/lib/galera/libgalera_enterprise_smm.so",
						Primary: PrimaryGalera{
							AutomaticFailover: ptr.To(false),
						},
						InitContainer: Container{
							Image:           "mariadb/mariadb-operator-enterprise:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						Agent: GaleraAgent{
							Image: "mariadb/mariadb-operator-enterprise:v0.0.26",
							KubernetesAuth: &KubernetesAuth{
								Enabled: false,
							},
						},
						AvailableWhenDonor: ptr.To(true),
						Recovery: &GaleraRecovery{
							Enabled:        true,
							MinClusterSize: ptr.To(intstr.FromString("33%")),
						},
					},
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						SST:            SSTRsync,
						GaleraLibPath:  "/usr/lib/galera/libgalera_enterprise_smm.so",
						ReplicaThreads: 1,
						InitContainer: Container{
							Image:           "mariadb/mariadb-operator-enterprise:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						AvailableWhenDonor: ptr.To(true),
						Config: GaleraConfig{
							ReuseStorageVolume: ptr.To(false),
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
								},
							},
						},
						Primary: PrimaryGalera{
							PodIndex:          ptr.To(0),
							AutomaticFailover: ptr.To(false),
						},
						Agent: GaleraAgent{
							Image:           "mariadb/mariadb-operator-enterprise:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Port:            5555,
							KubernetesAuth: &KubernetesAuth{
								Enabled: false,
							},
							GracefulShutdownTimeout: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
						},
						Recovery: &GaleraRecovery{
							Enabled:                 true,
							MinClusterSize:          ptr.To(intstr.FromString("33%")),
							ClusterMonitorInterval:  ptr.To(metav1.Duration{Duration: 10 * time.Second}),
							ClusterHealthyTimeout:   ptr.To(metav1.Duration{Duration: 30 * time.Second}),
							ClusterBootstrapTimeout: ptr.To(metav1.Duration{Duration: 10 * time.Minute}),
							PodRecoveryTimeout:      ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
							PodSyncTimeout:          ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
						},
					},
				},
				env,
			),
			Entry(
				"Recovery disabled",
				&MariaDB{
					ObjectMeta: mdbObjMeta,
					Spec: MariaDBSpec{
						Replicas: 3,
					},
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						Recovery: &GaleraRecovery{
							Enabled: false,
						},
					},
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						SST:            SSTMariaBackup,
						ReplicaThreads: 1,
						InitContainer: Container{
							Image:           "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						GaleraLibPath:      "/usr/lib/galera/libgalera_smm.so",
						AvailableWhenDonor: ptr.To(false),
						Config: GaleraConfig{
							ReuseStorageVolume: ptr.To(false),
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
								},
							},
						},
						Primary: PrimaryGalera{
							PodIndex:          ptr.To(0),
							AutomaticFailover: ptr.To(true),
						},
						Agent: GaleraAgent{
							Image:           "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Port:            5555,
							KubernetesAuth: &KubernetesAuth{
								Enabled: true,
							},
							GracefulShutdownTimeout: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
						},
						Recovery: &GaleraRecovery{
							Enabled: false,
						},
					},
				},
				env,
			),
			Entry(
				"InitJob anti-affinity",
				&MariaDB{
					ObjectMeta: mdbObjMeta,
					Spec: MariaDBSpec{
						Replicas: 3,
					},
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						Recovery: &GaleraRecovery{
							Enabled: false,
						},
						InitJob: &Job{
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
							},
						},
					},
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						SST:            SSTMariaBackup,
						ReplicaThreads: 1,
						InitContainer: Container{
							Image:           "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						GaleraLibPath:      "/usr/lib/galera/libgalera_smm.so",
						AvailableWhenDonor: ptr.To(false),
						Config: GaleraConfig{
							ReuseStorageVolume: ptr.To(false),
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
								},
							},
						},
						Primary: PrimaryGalera{
							PodIndex:          ptr.To(0),
							AutomaticFailover: ptr.To(true),
						},
						Agent: GaleraAgent{
							Image:           "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Port:            5555,
							KubernetesAuth: &KubernetesAuth{
								Enabled: true,
							},
							GracefulShutdownTimeout: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
						},
						Recovery: &GaleraRecovery{
							Enabled: false,
						},
						InitJob: &Job{
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
								Affinity: corev1.Affinity{
									PodAntiAffinity: &corev1.PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchExpressions: []metav1.LabelSelectorRequirement{
														{
															Key:      "app.kubernetes.io/instance",
															Operator: metav1.LabelSelectorOpIn,
															Values: []string{
																mdbObjMeta.Name,
															},
														},
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
							},
						},
					},
				},
				env,
			),
		)

		DescribeTable(
			"Has minimum cluster size",
			func(currentSize int, mdb *MariaDB, wantBool bool, wantErr bool) {
				clusterHasMinSize, err := mdb.Spec.Galera.Recovery.HasMinClusterSize(currentSize, mdb)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
				Expect(clusterHasMinSize).To(Equal(wantBool))
			},
			Entry(
				"Invalid min cluster size",
				1,
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									MinClusterSize: ptr.To(intstr.FromString("foo")),
								},
							},
						},
					},
				},
				false,
				true,
			),
			Entry(
				"Zero replicas",
				0,
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									MinClusterSize: ptr.To(intstr.FromString("50%")),
								},
							},
						},
					},
				},
				false,
				false,
			),
			Entry(
				"Less than min size",
				1,
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									MinClusterSize: ptr.To(intstr.FromString("50%")),
								},
							},
						},
					},
				},
				false,
				false,
			),
			Entry(
				"Exact min size",
				2,
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									MinClusterSize: ptr.To(intstr.FromString("50%")),
								},
							},
						},
					},
				},
				true,
				false,
			),
			Entry(
				"More than min size",
				3,
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:        true,
									MinClusterSize: ptr.To(intstr.FromString("50%")),
								},
							},
						},
					},
				},
				true,
				false,
			),
			Entry(
				"Even number of replicas",
				2,
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 4,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:        true,
									MinClusterSize: ptr.To(intstr.FromString("50%")),
								},
							},
						},
					},
				},
				true,
				false,
			),
		)

		DescribeTable(
			"Validate min cluster size",
			func(mdb *MariaDB, wantErr bool) {
				err := mdb.Spec.Galera.Recovery.Validate(mdb)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"No min cluster size",
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled: true,
								},
							},
						},
					},
				},
				false,
			),
			Entry(
				"Invalid min cluster size",
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:        true,
									MinClusterSize: ptr.To(intstr.FromString("foo")),
								},
							},
						},
					},
				},
				true,
			),
			Entry(
				"Disabled recovery",
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:        false,
									MinClusterSize: ptr.To(intstr.FromString("foo")),
								},
							},
						},
					},
				},
				false,
			),
			Entry(
				"Percentage",
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:        true,
									MinClusterSize: ptr.To(intstr.FromString("50%")),
								},
							},
						},
					},
				},
				false,
			),
			Entry(
				"Integer in range",
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:        true,
									MinClusterSize: ptr.To(intstr.FromInt(1)),
								},
							},
						},
					},
				},
				false,
			),
			Entry(
				"Integer negative",
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:        true,
									MinClusterSize: ptr.To(intstr.FromInt(-1)),
								},
							},
						},
					},
				},
				true,
			),
			Entry(
				"Integer out of range",
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:        true,
									MinClusterSize: ptr.To(intstr.FromInt(4)),
								},
							},
						},
					},
				},
				true,
			),
		)
	})
})
