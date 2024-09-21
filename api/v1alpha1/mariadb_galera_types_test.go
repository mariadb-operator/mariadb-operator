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
		WatchNamespace:           "",
		MariadbOperatorNamespace: testNamespace,
		MariadbOperatorImage:     "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
		MariadbGaleraLibPath:     "/usr/lib/galera/libgalera_smm.so",
	}
	Context("When creating a MariaDB Galera object", func() {
		mdbObjMeta := metav1.ObjectMeta{
			Name:      "mariadb-galera",
			Namespace: testNamespace,
		}
		DescribeTable("Should default",
			func(mdb *MariaDB, galera, expected *Galera, env *environment.OperatorEnv) {
				Expect(galera.SetDefaults(mdb, env)).To(Succeed())
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
							Image: "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
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
							Image: "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							Port:  5555,
							KubernetesAuth: &KubernetesAuth{
								Enabled: true,
							},
							GracefulShutdownTimeout: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
						},
						Recovery: &GaleraRecovery{
							Enabled:                 true,
							MinClusterSize:          ptr.To(intstr.FromInt(1)),
							ClusterMonitorInterval:  ptr.To(metav1.Duration{Duration: 10 * time.Second}),
							ClusterHealthyTimeout:   ptr.To(metav1.Duration{Duration: 30 * time.Second}),
							ClusterBootstrapTimeout: ptr.To(metav1.Duration{Duration: 10 * time.Minute}),
							PodRecoveryTimeout:      ptr.To(metav1.Duration{Duration: 5 * time.Minute}),
							PodSyncTimeout:          ptr.To(metav1.Duration{Duration: 5 * time.Minute}),
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
							Image: "mariadb/mariadb-operator-enterprise:v0.0.26",
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
							Image: "mariadb/mariadb-operator-enterprise:v0.0.26",
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
							Image: "mariadb/mariadb-operator-enterprise:v0.0.26",
							Port:  5555,
							KubernetesAuth: &KubernetesAuth{
								Enabled: false,
							},
							BasicAuth: &BasicAuth{
								Enabled:  true,
								Username: "mariadb-operator",
								PasswordSecretKeyRef: GeneratedSecretKeyRef{
									SecretKeySelector: corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "mariadb-galera-agent-auth",
										},
										Key: "password",
									},
									Generate: true,
								},
							},
							GracefulShutdownTimeout: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
						},
						Recovery: &GaleraRecovery{
							Enabled:                 true,
							MinClusterSize:          ptr.To(intstr.FromString("33%")),
							ClusterMonitorInterval:  ptr.To(metav1.Duration{Duration: 10 * time.Second}),
							ClusterHealthyTimeout:   ptr.To(metav1.Duration{Duration: 30 * time.Second}),
							ClusterBootstrapTimeout: ptr.To(metav1.Duration{Duration: 10 * time.Minute}),
							PodRecoveryTimeout:      ptr.To(metav1.Duration{Duration: 5 * time.Minute}),
							PodSyncTimeout:          ptr.To(metav1.Duration{Duration: 5 * time.Minute}),
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
							Image: "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
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
							Image: "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							Port:  5555,
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
		)

		DescribeTable("Should auto update data-plane",
			func(mdb *MariaDB, env *environment.OperatorEnv, galera *Galera, expectedImage string) {
				Expect(galera.SetDefaults(mdb, env)).ToNot(HaveOccurred())
				Expect(galera.Agent.Image).To(BeEquivalentTo(expectedImage))
				Expect(galera.InitContainer.Image).To(BeEquivalentTo(expectedImage))
			},
			Entry(
				"auto update disabled",
				&MariaDB{
					ObjectMeta: mdbObjMeta,
					Spec: MariaDBSpec{
						UpdateStrategy: UpdateStrategy{
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				&environment.OperatorEnv{
					MariadbOperatorImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.2",
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						Agent: GaleraAgent{
							Image: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
						},
						InitContainer: Container{
							Image: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
						},
					},
				},
				"docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			),
			Entry(
				"auto update enabled",
				&MariaDB{
					ObjectMeta: mdbObjMeta,
					Spec: MariaDBSpec{
						UpdateStrategy: UpdateStrategy{
							AutoUpdateDataPlane: ptr.To(true),
						},
					},
				},
				&environment.OperatorEnv{
					MariadbOperatorImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.2",
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						Agent: GaleraAgent{
							Image: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
						},
						InitContainer: Container{
							Image: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
						},
					},
				},
				"docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.2",
			),
		)

		DescribeTable("Has minimum cluster size",
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
									Enabled:        true,
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
				"Less than min fixed size",
				0,
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
				false,
			),
			Entry(
				"Exact min fixed size",
				1,
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
				true,
				false,
			),
			Entry(
				"More than min fixed size",
				3,
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:        true,
									MinClusterSize: ptr.To(intstr.FromInt(2)),
								},
							},
						},
					},
				},
				true,
				false,
			),
			Entry(
				"Less than min relative size",
				1,
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
				false,
			),
			Entry(
				"Exact min relative size",
				2,
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
				"More than min relative size",
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
			Entry(
				"Default min cluster size",
				1,
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
				true,
				false,
			),
		)

		DescribeTable("Validate",
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
									Enabled: false,
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
			Entry(
				"Invalid forceClusterBootstrapInPod",
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:                    true,
									ForceClusterBootstrapInPod: ptr.To("foo"),
								},
							},
						},
					},
				},
				true,
			),
			Entry(
				"Valid forceClusterBootstrapInPod",
				&MariaDB{
					Spec: MariaDBSpec{
						Replicas: 3,
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Recovery: &GaleraRecovery{
									Enabled:                    true,
									ForceClusterBootstrapInPod: ptr.To("mariadb-galera-0"),
								},
							},
						},
					},
				},
				false,
			),
		)
	})
})
