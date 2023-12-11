package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariaDB webhook", func() {
	Context("When creating a MariaDB", func() {
		meta := metav1.ObjectMeta{
			Name:      "mariadb-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(mdb *MariaDB, wantErr bool) {
				_ = k8sClient.Delete(testCtx, mdb)
				err := k8sClient.Create(testCtx, mdb)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"No source",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						BootstrapFrom: nil,
					},
				},
				false,
			),
			Entry(
				"Valid source",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						BootstrapFrom: &RestoreSource{
							BackupRef: &corev1.LocalObjectReference{
								Name: "backup-webhook",
							},
						},
					},
				},
				false,
			),
			Entry(
				"Invalid source",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						BootstrapFrom: &RestoreSource{
							FileName: func() *string { b := "backup.sql"; return &b }(),
						},
					},
				},
				true,
			),
			Entry(
				"Valid Galera",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
							GaleraSpec: GaleraSpec{
								SST:            &sst,
								ReplicaThreads: &replicaThreads,
							},
						},
						Replicas: 3,
					},
				},
				false,
			),
			Entry(
				"Valid replication",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replication: &Replication{
							ReplicationSpec: ReplicationSpec{
								Primary: &PrimaryReplication{
									PodIndex: func() *int { i := 0; return &i }(),
								},
								SyncBinlog: func() *bool { f := true; return &f }(),
							},
							Enabled: true,
						},
						Replicas: 3,
					},
				},
				false,
			),
			Entry(
				"Invalid HA",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replication: &Replication{
							ReplicationSpec: ReplicationSpec{
								Primary: &PrimaryReplication{
									PodIndex: func() *int { i := 0; return &i }(),
								},
								SyncBinlog: func() *bool { f := true; return &f }(),
							},
							Enabled: true,
						},
						Galera: &Galera{
							Enabled: true,
							GaleraSpec: GaleraSpec{
								SST:            &sst,
								ReplicaThreads: &replicaThreads,
							},
						},
						Replicas: 3,
					},
				},
				true,
			),
			Entry(
				"Fewer replicas required",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replicas: 4,
					},
				},
				true,
			),
			Entry(
				"More replicas required",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
							GaleraSpec: GaleraSpec{
								SST: &sst,
							},
						},
						Replicas: 1,
					},
				},
				true,
			),
			Entry(
				"Invalid SST",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
							GaleraSpec: GaleraSpec{
								SST: func() *SST {
									s := SST("foo")
									return &s
								}(),
								ReplicaThreads: &replicaThreads,
							},
						},
						Replicas: 3,
					},
				},
				true,
			),
			Entry(
				"Invalid replica threads",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
							GaleraSpec: GaleraSpec{
								SST: &sst,
								ReplicaThreads: func() *int {
									r := -1
									return &r
								}(),
							},
						},
						Replicas: 3,
					},
				},
				true,
			),
			Entry(
				"Invalid replication primary pod index",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replication: &Replication{
							ReplicationSpec: ReplicationSpec{
								Primary: &PrimaryReplication{
									PodIndex: func() *int { i := 4; return &i }(),
								},
								Replica: &ReplicaReplication{
									WaitPoint:         func() *WaitPoint { w := WaitPointAfterCommit; return &w }(),
									ConnectionTimeout: &metav1.Duration{Duration: time.Duration(1 * time.Second)},
									ConnectionRetries: func() *int { r := 3; return &r }(),
								},
							},
							Enabled: true,
						},
						Replicas: 3,
					},
				},
				true,
			),
			Entry(
				"Invalid Galera primary pod index",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							GaleraSpec: GaleraSpec{
								Primary: &PrimaryGalera{
									PodIndex: func() *int { i := 4; return &i }(),
								},
							},
							Enabled: true,
						},
						Replicas: 3,
					},
				},
				true,
			),
			Entry(
				"Invalid replica wait point",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replication: &Replication{
							ReplicationSpec: ReplicationSpec{
								Replica: &ReplicaReplication{
									WaitPoint: func() *WaitPoint { w := WaitPoint("foo"); return &w }(),
								},
							},
							Enabled: true,
						},
						Replicas: 3,
					},
				},
				true,
			),
			Entry(
				"Invalid GTID",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replication: &Replication{
							ReplicationSpec: ReplicationSpec{
								Replica: &ReplicaReplication{
									Gtid: func() *Gtid { g := Gtid("foo"); return &g }(),
								},
							},
							Enabled: true,
						},
						Replicas: 3,
					},
				},
				true,
			),
			Entry(
				"Invalid PodDisruptionBudget",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						PodDisruptionBudget: &PodDisruptionBudget{
							MaxUnavailable: func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
							MinAvailable:   func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
						},
					},
				},
				true,
			),
			Entry(
				"Valid PodDisruptionBudget",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						PodDisruptionBudget: &PodDisruptionBudget{
							MaxUnavailable: func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
						},
					},
				},
				false,
			),
		)

		It("Should default replication", func() {
			mariadb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-repl-default-webhook",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{
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
					Replicas: 3,
					Replication: &Replication{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariadb)).To(Succeed())
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mariadb), &mariadb)).To(Succeed())

			By("Expect MariaDB replication spec to be defaulted")
			Expect(mariadb.Spec.Replication.ReplicationSpec).To(Equal(DefaultReplicationSpec))
		})

		It("Should partially default replication", func() {
			mariadb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-repl-partial-default-webhook",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{
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
					Replicas: 3,
					Replication: &Replication{
						Enabled: true,
						ReplicationSpec: ReplicationSpec{
							Primary: &PrimaryReplication{
								PodIndex: func() *int { pi := 0; return &pi }(),
							},
							Replica: &ReplicaReplication{
								WaitPoint: func() *WaitPoint { w := WaitPointAfterSync; return &w }(),
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariadb)).To(Succeed())
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mariadb), &mariadb)).To(Succeed())

			By("Expect MariaDB replication spec to be defaulted")
			Expect(mariadb.Spec.Replication.ReplicationSpec).To(Equal(DefaultReplicationSpec))
		})

		It("Should partially default Galera", func() {
			mariadb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-galera-default-webhook",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{
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
					Replicas: 3,
					Galera: &Galera{
						Enabled: true,
						GaleraSpec: GaleraSpec{
							Agent: &GaleraAgent{
								Image:           "ghcr.io/mariadb-operator/agent:v0.0.3",
								ImagePullPolicy: corev1.PullIfNotPresent,
							},
							Recovery: &GaleraRecovery{
								Enabled: true,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariadb)).To(Succeed())
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mariadb), &mariadb)).To(Succeed())

			By("Expect MariaDB Galera spec to be defaulted")
			Expect(mariadb.Spec.Galera.GaleraSpec).To(Equal(DefaultGaleraSpec))
		})

		It("Should default Galera", func() {
			mariadb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-galera-partial-default-webhook",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{
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
					Replicas: 3,
					Galera: &Galera{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariadb)).To(Succeed())
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mariadb), &mariadb)).To(Succeed())

			By("Expect MariaDB Galera spec to be defaulted")
			Expect(mariadb.Spec.Galera.GaleraSpec).To(Equal(DefaultGaleraSpec))
		})
	})

	Context("When updating a MariaDB", Ordered, func() {
		key := types.NamespacedName{
			Name:      "mariadb-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			mariadb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-update-webhook",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{
					Image:           "mariadb:11.3.3",
					ImagePullPolicy: corev1.PullIfNotPresent,
					ContainerTemplate: ContainerTemplate{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
						Env: []corev1.EnvVar{
							{
								Name:  "TZ",
								Value: "SYSTEM",
							},
						},
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "mariadb",
									},
								},
							},
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret",
						},
						Key: "root-password",
					},
					Database: func() *string { t := "test"; return &t }(),
					Username: func() *string { t := "test"; return &t }(),
					PasswordSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret",
						},
						Key: "password",
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
					MyCnf: func() *string { c := "foo"; return &c }(),
					BootstrapFrom: &RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "backup",
						},
					},
					Metrics: &Metrics{
						Exporter: Exporter{
							Image:           "prom/mysqld-exporter:v0.14.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						ServiceMonitor: ServiceMonitor{
							PrometheusRelease: "prometheus",
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariadb)).To(Succeed())
		})
		DescribeTable(
			"Should validate",
			func(patchFn func(mdb *MariaDB), wantErr bool) {
				var mdb MariaDB
				Expect(k8sClient.Get(testCtx, key, &mdb)).To(Succeed())

				patch := client.MergeFrom(mdb.DeepCopy())
				patchFn(&mdb)

				err := k8sClient.Patch(testCtx, &mdb, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating RootPasswordSecretKeyRef",
				func(mdb *MariaDB) {
					mdb.Spec.RootPasswordSecretKeyRef.Key = "another-password"
				},
				true,
			),
			Entry(
				"Updating Database",
				func(mdb *MariaDB) {
					another := "another-database"
					mdb.Spec.Database = &another
				},
				true,
			),
			Entry(
				"Updating Username",
				func(mdb *MariaDB) {
					another := "another-username"
					mdb.Spec.Username = &another
				},
				true,
			),
			Entry(
				"Updating PasswordSecretKeyRef",
				func(mdb *MariaDB) {
					mdb.Spec.PasswordSecretKeyRef.Key = "another-password"
				},
				true,
			),
			Entry(
				"Updating Image",
				func(mdb *MariaDB) {
					mdb.Spec.Image = "mariadb:11.2.2"
				},
				false,
			),
			Entry(
				"Updating Port",
				func(mdb *MariaDB) {
					mdb.Spec.Port = 3307
				},
				false,
			),
			Entry(
				"Updating Storage",
				func(mdb *MariaDB) {
					newClass := "fast-storage"
					mdb.Spec.VolumeClaimTemplate.StorageClassName = &newClass
				},
				true,
			),
			Entry(
				"Updating MyCnf",
				func(mdb *MariaDB) {
					newCnf := "bar"
					mdb.Spec.MyCnf = &newCnf
				},
				true,
			),
			Entry(
				"Updating MyCnfConfigMapKeyRef",
				func(mdb *MariaDB) {
					mdb.Spec.MyCnfConfigMapKeyRef = &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-cnf-configmap",
						},
						Key: "config",
					}
				},
				false,
			),
			Entry(
				"Updating BootstrapFromBackup",
				func(mdb *MariaDB) {
					mdb.Spec.BootstrapFrom = nil
				},
				false,
			),
			Entry(
				"Updating Metrics",
				func(mdb *MariaDB) {
					mdb.Spec.Metrics.Exporter.Image = "prom/mysqld-exporter:v0.14.1"
				},
				false,
			),
			Entry(
				"Updating Resources",
				func(mdb *MariaDB) {
					mdb.Spec.Resources = &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("200m"),
						},
					}
				},
				false,
			),
			Entry(
				"Updating Env",
				func(mdb *MariaDB) {
					mdb.Spec.Env = []corev1.EnvVar{
						{
							Name:  "FOO",
							Value: "foo",
						},
					}
				},
				false,
			),
			Entry(
				"Updating EnvFrom",
				func(mdb *MariaDB) {
					mdb.Spec.EnvFrom = []corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb",
								},
							},
						},
					}
				},
				false,
			),
		)
	})

	Context("When updating MariaDB primary pod index", Ordered, func() {
		noSwitchoverKey := types.NamespacedName{
			Name:      "mariadb-no-switchover-webhook",
			Namespace: testNamespace,
		}
		switchoverKey := types.NamespacedName{
			Name:      "mariadb-switchover-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			test := "test"
			mariaDb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      noSwitchoverKey.Name,
					Namespace: noSwitchoverKey.Namespace,
				},
				Spec: MariaDBSpec{
					Database: &test,
					Username: &test,
					PasswordSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret",
						},
						Key: "password",
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
					Replication: &Replication{
						ReplicationSpec: ReplicationSpec{
							Primary: &PrimaryReplication{
								PodIndex:          func() *int { i := 0; return &i }(),
								AutomaticFailover: func() *bool { f := false; return &f }(),
							},
						},
						Enabled: true,
					},
					Replicas: 3,
				},
			}
			mariaDbSwitchover := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      switchoverKey.Name,
					Namespace: switchoverKey.Namespace,
				},
				Spec: MariaDBSpec{
					Database: &test,
					Username: &test,
					PasswordSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret",
						},
						Key: "password",
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
					Replication: &Replication{
						ReplicationSpec: ReplicationSpec{
							Primary: &PrimaryReplication{
								PodIndex:          func() *int { i := 0; return &i }(),
								AutomaticFailover: func() *bool { f := false; return &f }(),
							},
						},
						Enabled: true,
					},
					Replicas: 3,
				},
				Status: MariaDBStatus{
					Conditions: []metav1.Condition{
						{
							Type:    ConditionTypePrimarySwitched,
							Status:  metav1.ConditionFalse,
							Reason:  ConditionReasonSwitchPrimary,
							Message: "Switching primary",
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariaDb)).To(Succeed())
			Expect(k8sClient.Create(testCtx, &mariaDbSwitchover)).To(Succeed())

			By("Updating status conditions")
			Expect(k8sClient.Get(testCtx, switchoverKey, &mariaDbSwitchover)).To(Succeed())
			mariaDbSwitchover.Status.Conditions = []metav1.Condition{
				{
					Type:               ConditionTypePrimarySwitched,
					Status:             metav1.ConditionFalse,
					Reason:             ConditionReasonSwitchPrimary,
					Message:            "Switching primary",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(testCtx, &mariaDbSwitchover)).To(Succeed())
		})
		DescribeTable(
			"Should validate",
			func(key types.NamespacedName, patchFn func(mdb *MariaDB), wantErr bool) {
				var mdb MariaDB
				Expect(k8sClient.Get(testCtx, key, &mdb)).To(Succeed())

				patch := client.MergeFrom(mdb.DeepCopy())
				patchFn(&mdb)

				err := k8sClient.Patch(testCtx, &mdb, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating primary pod index",
				noSwitchoverKey,
				func(mdb *MariaDB) {
					i := 1
					mdb.Spec.Replication.Primary.PodIndex = &i
				},
				false,
			),
			Entry(
				"Updating automatic failover",
				noSwitchoverKey,
				func(mdb *MariaDB) {
					f := true
					mdb.Spec.Replication.Primary.AutomaticFailover = &f
				},
				false,
			),
			Entry(
				"Updating primary pod index when switching",
				switchoverKey,
				func(mdb *MariaDB) {
					i := 1
					mdb.Spec.Replication.Primary.PodIndex = &i
				},
				true,
			),
			Entry(
				"Updating automatic failover when switching",
				switchoverKey,
				func(mdb *MariaDB) {
					f := true
					mdb.Spec.Replication.Primary.AutomaticFailover = &f
				},
				true,
			),
		)
	})
})
