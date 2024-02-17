package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariaDB Galera types", func() {
	Context("When creating a MariaDB Galera object", func() {
		DescribeTable(
			"Should default",
			func(galera, expected *Galera) {
				galera.SetDefaults()
				Expect(galera).To(BeEquivalentTo(expected))
			},
			Entry(
				"Full default",
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
						VolumeClaimTemplate: VolumeClaimTemplate{
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
							ClusterHealthyTimeout:   ptr.To(metav1.Duration{Duration: 30 * time.Second}),
							ClusterBootstrapTimeout: ptr.To(metav1.Duration{Duration: 10 * time.Minute}),
							PodRecoveryTimeout:      ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
							PodSyncTimeout:          ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
						},
					},
				},
			),
			Entry(
				"Partial default",
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						SST: SSTRsync,
						Primary: PrimaryGalera{
							AutomaticFailover: ptr.To(false),
						},
						Agent: GaleraAgent{
							KubernetesAuth: &KubernetesAuth{
								Enabled: false,
							},
						},
					},
				},
				&Galera{
					Enabled: true,
					GaleraSpec: GaleraSpec{
						SST:            SSTRsync,
						ReplicaThreads: 1,
						InitContainer: Container{
							Image:           "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						VolumeClaimTemplate: VolumeClaimTemplate{
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
						Primary: PrimaryGalera{
							PodIndex:          ptr.To(0),
							AutomaticFailover: ptr.To(false),
						},
						Agent: GaleraAgent{
							Image:           "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Port:            5555,
							KubernetesAuth: &KubernetesAuth{
								Enabled: false,
							},
							GracefulShutdownTimeout: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
						},
						Recovery: &GaleraRecovery{
							Enabled:                 true,
							ClusterHealthyTimeout:   ptr.To(metav1.Duration{Duration: 30 * time.Second}),
							ClusterBootstrapTimeout: ptr.To(metav1.Duration{Duration: 10 * time.Minute}),
							PodRecoveryTimeout:      ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
							PodSyncTimeout:          ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
						},
					},
				},
			),
			Entry(
				"Recovery disabled",
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
						VolumeClaimTemplate: VolumeClaimTemplate{
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
			),
		)
	})
})
