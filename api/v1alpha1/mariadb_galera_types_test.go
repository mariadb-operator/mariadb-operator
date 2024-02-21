package v1alpha1

import (
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariaDB Galera types", func() {
	env := &environment.OperatorEnv{
		MariadbGaleraInitImage:  "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
		MariadbGaleraAgentImage: "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
		MariadbGaleraLibPath:    "/usr/lib/galera/libgalera_smm.so",
	}
	Context("When creating a MariaDB Galera object", func() {
		DescribeTable(
			"Should default",
			func(mdb *MariaDB, galera, expected *Galera, env *environment.OperatorEnv) {
				galera.SetDefaults(mdb, env)
				Expect(galera).To(BeEquivalentTo(expected))
			},
			Entry(
				"Full default",
				&MariaDB{
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
									Resources: corev1.ResourceRequirements{
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
							MinClusterSize:          ptr.To(int32(2)),
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
							MinClusterSize: ptr.To(int32(1)),
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
									Resources: corev1.ResourceRequirements{
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
							MinClusterSize:          ptr.To(int32(1)),
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
									Resources: corev1.ResourceRequirements{
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
		)
	})
})
