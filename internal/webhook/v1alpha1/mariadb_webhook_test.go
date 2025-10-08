package v1alpha1

import (
	"time"

	"github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("v1alpha1.MariaDB webhook", func() {
	Context("When creating a v1alpha1.MariaDB", func() {
		meta := metav1.ObjectMeta{
			Name:      "mariadb-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(mdb *v1alpha1.MariaDB, wantErr bool) {
				_ = k8sClient.Delete(testCtx, mdb)
				err := k8sClient.Create(testCtx, mdb)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"No BootstrapFrom",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						BootstrapFrom: nil,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
			),
			Entry(
				"Valid BootstrapFrom",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						BootstrapFrom: &v1alpha1.BootstrapFrom{
							BackupRef: &v1alpha1.TypedLocalObjectReference{
								Name: "backup-webhook",
							},
						},
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
			),
			Entry(
				"Invalid BootstrapFrom",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						BootstrapFrom: &v1alpha1.BootstrapFrom{
							TargetRecoveryTime: &metav1.Time{Time: time.Now()},
						},
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Valid Galera",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Galera: &v1alpha1.Galera{
							Enabled: true,
							GaleraSpec: v1alpha1.GaleraSpec{
								SST:            v1alpha1.SSTMariaBackup,
								ReplicaThreads: 1,
							},
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
			),
			Entry(
				"Valid replication",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Replication: &v1alpha1.Replication{
							ReplicationSpec: v1alpha1.ReplicationSpec{
								Primary: v1alpha1.PrimaryReplication{
									PodIndex: func() *int { i := 0; return &i }(),
								},
								SyncBinlog: ptr.To(1),
							},
							Enabled: true,
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
			),
			Entry(
				"Invalid HA",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Replication: &v1alpha1.Replication{
							ReplicationSpec: v1alpha1.ReplicationSpec{
								Primary: v1alpha1.PrimaryReplication{
									PodIndex: func() *int { i := 0; return &i }(),
								},
								SyncBinlog: ptr.To(1),
							},
							Enabled: true,
						},
						Galera: &v1alpha1.Galera{
							Enabled: true,
							GaleraSpec: v1alpha1.GaleraSpec{
								SST:            v1alpha1.SSTMariaBackup,
								ReplicaThreads: 1,
							},
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Fewer replicas required",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Replicas: 4,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"More replicas required",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Galera: &v1alpha1.Galera{
							Enabled: true,
							GaleraSpec: v1alpha1.GaleraSpec{
								SST: v1alpha1.SSTMariaBackup,
							},
						},
						Replicas: 1,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid min cluster size",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Galera: &v1alpha1.Galera{
							Enabled: true,
							GaleraSpec: v1alpha1.GaleraSpec{
								SST: v1alpha1.SSTMariaBackup,
								Recovery: &v1alpha1.GaleraRecovery{
									Enabled:        true,
									MinClusterSize: ptr.To(intstr.FromInt(4)),
								},
							},
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid force cluster bootstrap",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Galera: &v1alpha1.Galera{
							Enabled: true,
							GaleraSpec: v1alpha1.GaleraSpec{
								SST: v1alpha1.SSTMariaBackup,
								Recovery: &v1alpha1.GaleraRecovery{
									Enabled:                    true,
									ForceClusterBootstrapInPod: ptr.To("foo"),
								},
							},
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid SST",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Galera: &v1alpha1.Galera{
							Enabled: true,
							GaleraSpec: v1alpha1.GaleraSpec{
								SST:            v1alpha1.SST("foo"),
								ReplicaThreads: 1,
							},
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid replica threads",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Galera: &v1alpha1.Galera{
							Enabled: true,
							GaleraSpec: v1alpha1.GaleraSpec{
								SST:            v1alpha1.SSTMariaBackup,
								ReplicaThreads: -1,
							},
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid provider options",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Galera: &v1alpha1.Galera{
							Enabled: true,
							GaleraSpec: v1alpha1.GaleraSpec{
								SST: v1alpha1.SSTMariaBackup,
								ProviderOptions: map[string]string{
									"ist.recv_addr": "1.2.3.4:4568",
								},
							},
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid agent auth",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Galera: &v1alpha1.Galera{
							Enabled: true,
							GaleraSpec: v1alpha1.GaleraSpec{
								Agent: v1alpha1.Agent{
									BasicAuth: &v1alpha1.BasicAuth{
										Enabled: true,
									},
									KubernetesAuth: &v1alpha1.KubernetesAuth{
										Enabled: true,
									},
								},
							},
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid replication primary pod index",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Replication: &v1alpha1.Replication{
							ReplicationSpec: v1alpha1.ReplicationSpec{
								Primary: v1alpha1.PrimaryReplication{
									PodIndex: func() *int { i := 4; return &i }(),
								},
								Replica: v1alpha1.ReplicaReplication{
									WaitPoint:         ptr.To(v1alpha1.WaitPointAfterCommit),
									ConnectionTimeout: &metav1.Duration{Duration: time.Duration(1 * time.Second)},
									ConnectionRetries: ptr.To(3),
								},
							},
							Enabled: true,
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid Galera primary pod index",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Galera: &v1alpha1.Galera{
							GaleraSpec: v1alpha1.GaleraSpec{
								Primary: v1alpha1.PrimaryGalera{
									PodIndex: ptr.To(4),
								},
							},
							Enabled: true,
						},
						Replicas: 3,
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid replica wait point",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Replication: &v1alpha1.Replication{
							ReplicationSpec: v1alpha1.ReplicationSpec{
								Replica: v1alpha1.ReplicaReplication{
									WaitPoint: ptr.To(v1alpha1.WaitPoint("foo")),
								},
							},
							Enabled: true,
						},
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
						Replicas: 3,
					},
				},
				true,
			),
			Entry(
				"Invalid GTID",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Replication: &v1alpha1.Replication{
							ReplicationSpec: v1alpha1.ReplicationSpec{
								Replica: v1alpha1.ReplicaReplication{
									Gtid: ptr.To(v1alpha1.Gtid("foo")),
								},
							},
							Enabled: true,
						},
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
						Replicas: 3,
					},
				},
				true,
			),
			Entry(
				"Invalid MaxScale",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						MaxScaleRef: &v1alpha1.ObjectReference{
							Name: "maxscale",
						},
						MaxScale: &v1alpha1.MariaDBMaxScaleSpec{
							Enabled: true,
						},
					},
				},
				true,
			),
			Entry(
				"Invalid PodDisruptionBudget",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						PodDisruptionBudget: &v1alpha1.PodDisruptionBudget{
							MaxUnavailable: ptr.To(intstr.FromString("50%")),
							MinAvailable:   ptr.To(intstr.FromString("50%")),
						},
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Valid PodDisruptionBudget",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						PodDisruptionBudget: &v1alpha1.PodDisruptionBudget{
							MaxUnavailable: ptr.To(intstr.FromString("50%")),
						},
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
			),
			Entry(
				"Invalid storage",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						Storage: v1alpha1.Storage{},
					},
				},
				true,
			),
			Entry(
				"Invalid rootPasswordSecretKeyRef and rootEmptyPassword",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						RootPasswordSecretKeyRef: v1alpha1.GeneratedSecretKeyRef{
							SecretKeySelector: v1alpha1.SecretKeySelector{
								LocalObjectReference: v1alpha1.LocalObjectReference{
									Name: "secret",
								},
								Key: "root-password",
							},
						},
						RootEmptyPassword: ptr.To(true),
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Valid rootPasswordSecretKeyRef",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						RootPasswordSecretKeyRef: v1alpha1.GeneratedSecretKeyRef{
							SecretKeySelector: v1alpha1.SecretKeySelector{
								LocalObjectReference: v1alpha1.LocalObjectReference{
									Name: "secret",
								},
								Key: "root-password",
							},
						},
						RootEmptyPassword: ptr.To(false),
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
			),
			Entry(
				"Valid rootEmptyPassword",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						RootPasswordSecretKeyRef: v1alpha1.GeneratedSecretKeyRef{},
						RootEmptyPassword:        ptr.To(true),
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
			),
			Entry(
				"Invalid TLS",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						RootPasswordSecretKeyRef: v1alpha1.GeneratedSecretKeyRef{
							SecretKeySelector: v1alpha1.SecretKeySelector{
								LocalObjectReference: v1alpha1.LocalObjectReference{
									Name: "secret",
								},
								Key: "root-password",
							},
						},
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
						TLS: &v1alpha1.TLS{
							Enabled: true,
							ServerCertSecretRef: &v1alpha1.LocalObjectReference{
								Name: "server-cert",
							},
						},
					},
				},
				true,
			),
			Entry(
				"Valid TLS",
				&v1alpha1.MariaDB{
					ObjectMeta: meta,
					Spec: v1alpha1.MariaDBSpec{
						RootPasswordSecretKeyRef: v1alpha1.GeneratedSecretKeyRef{
							SecretKeySelector: v1alpha1.SecretKeySelector{
								LocalObjectReference: v1alpha1.LocalObjectReference{
									Name: "secret",
								},
								Key: "root-password",
							},
						},
						Storage: v1alpha1.Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
						TLS: &v1alpha1.TLS{
							Enabled: true,
							ServerCASecretRef: &v1alpha1.LocalObjectReference{
								Name: "server-ca",
							},
							ServerCertSecretRef: &v1alpha1.LocalObjectReference{
								Name: "server-cert",
							},
						},
					},
				},
				false,
			),
		)
	})

	Context("When updating a v1alpha1.MariaDB", Ordered, func() {
		key := types.NamespacedName{
			Name:      "mariadb-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			mariadb := v1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.MariaDBSpec{
					Image:           "mariadb:11.3.3",
					ImagePullPolicy: corev1.PullIfNotPresent,
					PodTemplate: v1alpha1.PodTemplate{
						PriorityClassName: ptr.To("PriorityClassName"),
					},
					ContainerTemplate: v1alpha1.ContainerTemplate{
						Resources: &v1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
						Env: []v1alpha1.EnvVar{
							{
								Name:  "TZ",
								Value: "SYSTEM",
							},
						},
						EnvFrom: []v1alpha1.EnvFromSource{
							{
								ConfigMapRef: &v1alpha1.LocalObjectReference{
									Name: "mariadb",
								},
							},
						},
					},
					RootPasswordSecretKeyRef: v1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "secret",
							},
							Key: "root-password",
						},
					},
					Database: ptr.To("test"),
					Username: ptr.To("test"),
					PasswordSecretKeyRef: &v1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "secret",
							},
							Key: "password",
						},
					},
					MyCnf: ptr.To("foo"),
					BootstrapFrom: &v1alpha1.BootstrapFrom{
						BackupRef: &v1alpha1.TypedLocalObjectReference{
							Name: "backup",
						},
					},
					Metrics: &v1alpha1.MariadbMetrics{
						Exporter: v1alpha1.Exporter{
							Image:           "prom/mysqld-exporter:v0.15.1",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						ServiceMonitor: v1alpha1.ServiceMonitor{
							PrometheusRelease: "prometheus",
						},
					},
					Storage: v1alpha1.Storage{
						Size: ptr.To(resource.MustParse("100Mi")),
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariadb)).To(Succeed())
		})
		DescribeTable(
			"Should validate",
			func(patchFn func(mdb *v1alpha1.MariaDB), wantErr bool) {
				var mdb v1alpha1.MariaDB
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
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.RootPasswordSecretKeyRef.Key = "another-password"
				},
				true,
			),
			Entry(
				"Updating Database",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.Database = ptr.To("another-database")
				},
				true,
			),
			Entry(
				"Updating Username",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.Username = ptr.To("another-username")
				},
				true,
			),
			Entry(
				"Updating PasswordSecretKeyRef",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.PasswordSecretKeyRef.Key = "another-password"
				},
				false,
			),
			Entry(
				"Updating Image",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.Image = "mariadb:11.2.2"
				},
				false,
			),
			Entry(
				"Updating Port",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.Port = 3307
				},
				false,
			),
			Entry(
				"Updating Storage size",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.Storage.Size = ptr.To(resource.MustParse("200Mi"))
				},
				false,
			),
			Entry(
				"Decreasing Storage size",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.Storage.Size = ptr.To(resource.MustParse("50Mi"))
				},
				true,
			),
			Entry(
				"Updating MyCnf",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.MyCnf = ptr.To("bar")
				},
				false,
			),
			Entry(
				"Updating MyCnfConfigMapKeyRef",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.MyCnfConfigMapKeyRef = &v1alpha1.ConfigMapKeySelector{
						LocalObjectReference: v1alpha1.LocalObjectReference{
							Name: "my-cnf-configmap",
						},
						Key: "config",
					}
				},
				false,
			),
			Entry(
				"Updating BootstrapFrom",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.BootstrapFrom = nil
				},
				false,
			),
			Entry(
				"Updating Metrics",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.Metrics.Exporter.Image = "prom/mysqld-exporter:v0.14.1"
				},
				false,
			),
			Entry(
				"Updating Resources",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.Resources = &v1alpha1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("200m"),
						},
					}
				},
				false,
			),
			Entry(
				"Updating Env",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.Env = []v1alpha1.EnvVar{
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
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.EnvFrom = []v1alpha1.EnvFromSource{
						{
							ConfigMapRef: &v1alpha1.LocalObjectReference{
								Name: "mariadb",
							},
						},
					}
				},
				false,
			),
			Entry(
				"Updating to invalid TLS",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.TLS = &v1alpha1.TLS{
						Enabled: true,
						ServerCertSecretRef: &v1alpha1.LocalObjectReference{
							Name: "server-cert",
						},
					}
				},
				true,
			),
			Entry(
				"Updating to valid TLS",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.TLS = &v1alpha1.TLS{
						Enabled: true,
						ServerCASecretRef: &v1alpha1.LocalObjectReference{
							Name: "server-ca",
						},
						ServerCertSecretRef: &v1alpha1.LocalObjectReference{
							Name: "server-cert",
						},
					}
				},
				false,
			),
			Entry(
				"Updating PriorityClassName",
				func(mdb *v1alpha1.MariaDB) {
					mdb.Spec.PriorityClassName = ptr.To("new-PriorityClassName")
				},
				false,
			),
		)
	})

	Context("When updating v1alpha1.MariaDB primary pod index", Ordered, func() {
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
			mariaDb := v1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      noSwitchoverKey.Name,
					Namespace: noSwitchoverKey.Namespace,
				},
				Spec: v1alpha1.MariaDBSpec{
					Database: &test,
					Username: &test,
					PasswordSecretKeyRef: &v1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "secret",
							},
							Key: "password",
						},
					},
					Replication: &v1alpha1.Replication{
						ReplicationSpec: v1alpha1.ReplicationSpec{
							Primary: v1alpha1.PrimaryReplication{
								PodIndex:          func() *int { i := 0; return &i }(),
								AutomaticFailover: func() *bool { f := false; return &f }(),
							},
						},
						Enabled: true,
					},
					Replicas: 3,
					Storage: v1alpha1.Storage{
						Size: ptr.To(resource.MustParse("100Mi")),
					},
				},
			}
			mariaDbSwitchover := v1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      switchoverKey.Name,
					Namespace: switchoverKey.Namespace,
				},
				Spec: v1alpha1.MariaDBSpec{
					Database: &test,
					Username: &test,
					PasswordSecretKeyRef: &v1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "secret",
							},
							Key: "password",
						},
					},
					Replication: &v1alpha1.Replication{
						ReplicationSpec: v1alpha1.ReplicationSpec{
							Primary: v1alpha1.PrimaryReplication{
								PodIndex:          func() *int { i := 0; return &i }(),
								AutomaticFailover: func() *bool { f := false; return &f }(),
							},
						},
						Enabled: true,
					},
					Replicas: 3,
					Storage: v1alpha1.Storage{
						Size: ptr.To(resource.MustParse("100Mi")),
					},
				},
				Status: v1alpha1.MariaDBStatus{
					Conditions: []metav1.Condition{
						{
							Type:    v1alpha1.ConditionTypePrimarySwitched,
							Status:  metav1.ConditionFalse,
							Reason:  v1alpha1.ConditionReasonSwitchPrimary,
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
					Type:               v1alpha1.ConditionTypePrimarySwitched,
					Status:             metav1.ConditionFalse,
					Reason:             v1alpha1.ConditionReasonSwitchPrimary,
					Message:            "Switching primary",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(testCtx, &mariaDbSwitchover)).To(Succeed())
		})
		DescribeTable(
			"Should validate",
			func(key types.NamespacedName, patchFn func(mdb *v1alpha1.MariaDB), wantErr bool) {
				var mdb v1alpha1.MariaDB
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
				func(mdb *v1alpha1.MariaDB) {
					i := 1
					mdb.Spec.Replication.Primary.PodIndex = &i
				},
				false,
			),
			Entry(
				"Updating automatic failover",
				noSwitchoverKey,
				func(mdb *v1alpha1.MariaDB) {
					f := true
					mdb.Spec.Replication.Primary.AutomaticFailover = &f
				},
				false,
			),
			Entry(
				"Updating primary pod index when switching",
				switchoverKey,
				func(mdb *v1alpha1.MariaDB) {
					i := 1
					mdb.Spec.Replication.Primary.PodIndex = &i
				},
				true,
			),
			Entry(
				"Updating automatic failover when switching",
				switchoverKey,
				func(mdb *v1alpha1.MariaDB) {
					f := true
					mdb.Spec.Replication.Primary.AutomaticFailover = &f
				},
				true,
			),
		)
	})
})
