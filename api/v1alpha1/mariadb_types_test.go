package v1alpha1

import (
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariaDB types", func() {
	format.MaxLength = 8000
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-obj",
		Namespace: "mariadb-obj",
	}
	env := &environment.OperatorEnv{
		RelatedMariadbImage: "mariadb:11.0.3",
	}
	Context("When creating a MariaDB object", func() {
		DescribeTable(
			"Should default",
			func(mdb, expected *MariaDB, env *environment.OperatorEnv) {
				Expect(mdb.SetDefaults(env)).To(Succeed())
				Expect(mdb).To(BeEquivalentTo(expected))
			},
			Entry(
				"Empty",
				&MariaDB{
					ObjectMeta: objMeta,
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"Image, root password and port",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image: "mariadb:lts",
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "root",
								},
								Key: "pwd",
							},
						},
						Port: 3307,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             "mariadb:lts",
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "root",
								},
								Key: "pwd",
							},
						},
						Port: 3307,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"Root password empty & port",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             "mariadb:lts",
						RootEmptyPassword: ptr.To(true),
						Port:              3307,
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             "mariadb:lts",
						RootEmptyPassword: ptr.To(true),
						Port:              3307,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"Bootstrap from",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						BootstrapFrom: &BootstrapFrom{
							BackupRef: &TypedLocalObjectReference{
								Name: "test",
								Kind: PhysicalBackupKind,
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						BootstrapFrom: &BootstrapFrom{
							BackupRef: &TypedLocalObjectReference{
								Name: "test",
								Kind: PhysicalBackupKind,
							},
							BackupContentType: BackupContentTypePhysical,
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"my.cnf",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						MyCnf: ptr.To(`
						[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M
						`),
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						MyCnf: ptr.To(`
						[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M
						`),
						MyCnfConfigMapKeyRef: &ConfigMapKeySelector{
							LocalObjectReference: LocalObjectReference{
								Name: "mariadb-obj-config",
							},
							Key: "my.cnf",
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"my.cnf and reference",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						MyCnf: ptr.To(`
						[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M
						`),
						MyCnfConfigMapKeyRef: &ConfigMapKeySelector{
							LocalObjectReference: LocalObjectReference{
								Name: "mariadb-config",
							},
							Key: "mariadb.cnf",
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						MyCnf: ptr.To(`
						[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M
						`),
						MyCnfConfigMapKeyRef: &ConfigMapKeySelector{
							LocalObjectReference: LocalObjectReference{
								Name: "mariadb-config",
							},
							Key: "mariadb.cnf",
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"user and database",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Username: ptr.To("user"),
						Database: ptr.To("test"),
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port:     3306,
						Username: ptr.To("user"),
						Database: ptr.To("test"),
						PasswordSecretKeyRef: &GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-password",
								},
								Key: "password",
							},
							Generate: true,
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"user, database and password",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Username: ptr.To("user"),
						Database: ptr.To("test"),
						PasswordSecretKeyRef: &GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "user-password",
								},
								Key: "pwd",
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port:     3306,
						Username: ptr.To("user"),
						Database: ptr.To("test"),
						PasswordSecretKeyRef: &GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "user-password",
								},
								Key: "pwd",
							},
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"metrics",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Metrics: &MariadbMetrics{
							Enabled: true,
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Metrics: &MariadbMetrics{
							Enabled: true,
							Exporter: Exporter{
								Image: env.RelatedExporterImage,
								Port:  9104,
							},
							Username: "mariadb-obj-metrics",
							PasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "mariadb-obj-metrics-password",
									},
									Key: "password",
								},
								Generate: true,
							},
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"metrics with anti-affinity",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Metrics: &MariadbMetrics{
							Enabled: true,
							Exporter: Exporter{
								Affinity: &AffinityConfig{
									AntiAffinityEnabled: ptr.To(true),
								},
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Metrics: &MariadbMetrics{
							Enabled: true,
							Exporter: Exporter{
								Image: env.RelatedExporterImage,
								Port:  9104,
								Affinity: &AffinityConfig{
									AntiAffinityEnabled: ptr.To(true),
									Affinity: Affinity{
										PodAntiAffinity: &PodAntiAffinity{
											RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
												{
													LabelSelector: &LabelSelector{
														MatchExpressions: []LabelSelectorRequirement{
															{
																Key:      "app.kubernetes.io/instance",
																Operator: metav1.LabelSelectorOpIn,
																Values: []string{
																	objMeta.Name,
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
							Username: "mariadb-obj-metrics",
							PasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "mariadb-obj-metrics-password",
									},
									Key: "password",
								},
								Generate: true,
							},
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"MaxScale",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						MaxScale: &MariaDBMaxScaleSpec{
							Enabled: true,
							Services: []MaxScaleService{
								{
									Name:   "rw-router",
									Router: ServiceRouterReadWriteSplit,
									Listener: MaxScaleListener{
										Port: 3306,
									},
								},
							},
							Monitor: &MaxScaleMonitor{
								Module: MonitorModuleMariadb,
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						MaxScaleRef: &ObjectReference{
							Name:      "mariadb-obj-maxscale",
							Namespace: "mariadb-obj",
						},
						MaxScale: &MariaDBMaxScaleSpec{
							Enabled: true,
							Services: []MaxScaleService{
								{
									Name:   "rw-router",
									Router: ServiceRouterReadWriteSplit,
									Listener: MaxScaleListener{
										Port: 3306,
									},
								},
							},
							Monitor: &MaxScaleMonitor{
								Module: MonitorModuleMariadb,
							},
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"storage",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Storage: Storage{
							Ephemeral:          ptr.To(false),
							ResizeInUseVolumes: ptr.To(true),
							Size:               ptr.To(resource.MustParse("100Mi")),
							StorageClassName:   "my-class",
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
							Size:                ptr.To(resource.MustParse("100Mi")),
							StorageClassName:    "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
									StorageClassName: ptr.To("my-class"),
								},
							},
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"storage drift",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:          ptr.To(false),
							ResizeInUseVolumes: ptr.To(true),
							Size:               ptr.To(resource.MustParse("100Mi")),
							StorageClassName:   "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("200Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
									StorageClassName: ptr.To("another-class"),
								},
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
							Size:                ptr.To(resource.MustParse("100Mi")),
							StorageClassName:    "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
									StorageClassName: ptr.To("my-class"),
								},
							},
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
						TLS: &TLS{
							Enabled: true,
						},
					},
				},
				env,
			),
			Entry(
				"TLS",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						TLS: &TLS{
							Enabled: true,
							ServerCASecretRef: &LocalObjectReference{
								Name: "server-ca",
							},
							ServerCertSecretRef: &LocalObjectReference{
								Name: "server-cert",
							},
							ClientCASecretRef: &LocalObjectReference{
								Name: "client-ca",
							},
							ClientCertSecretRef: &LocalObjectReference{
								Name: "client-cert",
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
							ServerCASecretRef: &LocalObjectReference{
								Name: "server-ca",
							},
							ServerCertSecretRef: &LocalObjectReference{
								Name: "server-cert",
							},
							ClientCASecretRef: &LocalObjectReference{
								Name: "client-ca",
							},
							ClientCertSecretRef: &LocalObjectReference{
								Name: "client-cert",
							},
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"updates",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						UpdateStrategy: UpdateStrategy{
							Type:                OnDeleteUpdateType,
							AutoUpdateDataPlane: ptr.To(true),
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                OnDeleteUpdateType,
							AutoUpdateDataPlane: ptr.To(true),
						},
					},
				},
				env,
			),
			Entry(
				"ephemeral storage",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Storage: Storage{
							Ephemeral: ptr.To(true),
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
					},
				},
				env,
			),
			Entry(
				"affinity",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						PodTemplate: PodTemplate{
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "mariadb-obj-root",
								},
								Key: "password",
							},
							Generate: true,
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
						},
						TLS: &TLS{
							Enabled: true,
						},
						UpdateStrategy: UpdateStrategy{
							Type:                ReplicasFirstPrimaryLastUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
								Affinity: Affinity{
									PodAntiAffinity: &PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
											{
												LabelSelector: &LabelSelector{
													MatchExpressions: []LabelSelectorRequirement{
														{
															Key:      "app.kubernetes.io/instance",
															Operator: metav1.LabelSelectorOpIn,
															Values: []string{
																objMeta.Name,
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
			Entry(
				"Full",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image: "mariadb:lts",
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "root",
								},
								Key: "pwd",
							},
						},
						Port:     3307,
						Username: ptr.To("user"),
						Database: ptr.To("test"),
						PasswordSecretKeyRef: &GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "user-password",
								},
								Key: "pwd",
							},
						},
						MyCnf: ptr.To(`
						[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M
						`),
						MyCnfConfigMapKeyRef: &ConfigMapKeySelector{
							LocalObjectReference: LocalObjectReference{
								Name: "mariadb-config",
							},
							Key: "mariadb.cnf",
						},
						Metrics: &MariadbMetrics{
							Enabled: true,
						},
						TLS: &TLS{
							Enabled: true,
							ServerCASecretRef: &LocalObjectReference{
								Name: "server-ca",
							},
							ServerCertSecretRef: &LocalObjectReference{
								Name: "server-cert",
							},
							ClientCASecretRef: &LocalObjectReference{
								Name: "client-ca",
							},
							ClientCertSecretRef: &LocalObjectReference{
								Name: "client-cert",
							},
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
							Size:                ptr.To(resource.MustParse("100Mi")),
							StorageClassName:    "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
									StorageClassName: ptr.To("my-class"),
								},
							},
						},
						UpdateStrategy: UpdateStrategy{
							Type:                OnDeleteUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
						PodTemplate: PodTemplate{
							ServiceAccountName: ptr.To("mariadb-sa"),
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
								Affinity: Affinity{
									PodAntiAffinity: &PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
											{
												LabelSelector: &LabelSelector{
													MatchExpressions: []LabelSelectorRequirement{
														{
															Key:      "app.kubernetes.io/instance",
															Operator: metav1.LabelSelectorOpIn,
															Values: []string{
																objMeta.Name,
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
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             "mariadb:lts",
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "root",
								},
								Key: "pwd",
							},
						},
						Port:     3307,
						Username: ptr.To("user"),
						Database: ptr.To("test"),
						PasswordSecretKeyRef: &GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "user-password",
								},
								Key: "pwd",
							},
						},
						MyCnf: ptr.To(`
						[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M
						`),
						MyCnfConfigMapKeyRef: &ConfigMapKeySelector{
							LocalObjectReference: LocalObjectReference{
								Name: "mariadb-config",
							},
							Key: "mariadb.cnf",
						},
						Metrics: &MariadbMetrics{
							Enabled: true,
							Exporter: Exporter{
								Image: env.RelatedExporterImage,
								Port:  9104,
							},
							Username: "mariadb-obj-metrics",
							PasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "mariadb-obj-metrics-password",
									},
									Key: "password",
								},
								Generate: true,
							},
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
							Size:                ptr.To(resource.MustParse("100Mi")),
							StorageClassName:    "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
									StorageClassName: ptr.To("my-class"),
								},
							},
						},
						TLS: &TLS{
							Enabled: true,
							ServerCASecretRef: &LocalObjectReference{
								Name: "server-ca",
							},
							ServerCertSecretRef: &LocalObjectReference{
								Name: "server-cert",
							},
							ClientCASecretRef: &LocalObjectReference{
								Name: "client-ca",
							},
							ClientCertSecretRef: &LocalObjectReference{
								Name: "client-cert",
							},
						},
						UpdateStrategy: UpdateStrategy{
							Type:                OnDeleteUpdateType,
							AutoUpdateDataPlane: ptr.To(false),
						},
						PodTemplate: PodTemplate{
							ServiceAccountName: ptr.To("mariadb-sa"),
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
								Affinity: Affinity{
									PodAntiAffinity: &PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
											{
												LabelSelector: &LabelSelector{
													MatchExpressions: []LabelSelectorRequirement{
														{
															Key:      "app.kubernetes.io/instance",
															Operator: metav1.LabelSelectorOpIn,
															Values: []string{
																objMeta.Name,
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
			"Validate storage",
			func(mdb *MariaDB, wantErr bool) {
				err := mdb.Spec.Storage.Validate(mdb)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Empty",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{},
					},
				},
				true,
			),
			Entry(
				"Ephemeral and HA",
				&MariaDB{
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
						},
						Storage: Storage{
							Ephemeral: ptr.To(true),
						},
					},
				},
				true,
			),
			Entry(
				"Ephemeral and regular",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							Ephemeral: ptr.To(true),
							Size:      ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Ephemeral",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							Ephemeral: ptr.To(true),
						},
					},
				},
				false,
			),
			Entry(
				"Zero size",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							Size: ptr.To(resource.MustParse("0Mi")),
						},
					},
				},
				true,
			),
			Entry(
				"Size",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
			),
			Entry(
				"VolumeClaimTemplate",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
								},
							},
						},
					},
				},
				false,
			),
			Entry(
				"Size and VolumeClaimTemplate",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
								},
							},
						},
					},
				},
				false,
			),
			Entry(
				"Storage decrease",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							Size: ptr.To(resource.MustParse("50Mi")),
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
								},
							},
						},
					},
				},
				true,
			),
			Entry(
				"Storage increase",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							Size: ptr.To(resource.MustParse("150Mi")),
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
								},
							},
						},
					},
				},
				false,
			),
		)

		DescribeTable(
			"Get size",
			func(mdb *MariaDB, wantSize *resource.Quantity) {
				Expect(mdb.Spec.Storage.GetSize()).To(Equal(wantSize))
			},
			Entry(
				"Empty",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{},
					},
				},
				nil,
			),
			Entry(
				"No storage",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU: resource.MustParse("100Mi"),
										},
									},
								},
							},
						},
					},
				},
				nil,
			),
			Entry(
				"From VolumeClaimTemplate",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
								},
							},
						},
					},
				},
				ptr.To(resource.MustParse("100Mi")),
			),
			Entry(
				"From Size",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				ptr.To(resource.MustParse("100Mi")),
			),
		)
	})

	Context("When creating a BootstrapFrom object", func() {
		DescribeTable(
			"Should validate",
			func(bootstrapFrom *BootstrapFrom, wantErr bool) {
				if wantErr {
					Expect(bootstrapFrom.Validate()).ToNot(Succeed())
				} else {
					Expect(bootstrapFrom.Validate()).To(Succeed())
				}
			},
			Entry(
				"No bootstrap source",
				&BootstrapFrom{},
				true,
			),
			Entry(
				"Invalid backup kind",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: "test",
					},
				},
				true,
			),
			Entry(
				"Invalid backup content type",
				&BootstrapFrom{
					BackupContentType: BackupContentType("test"),
				},
				true,
			),
			Entry(
				"Inconsistent backup type 1",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
					},
					BackupContentType: BackupContentTypePhysical,
				},
				true,
			),
			Entry(
				"Inconsistent backup type 2",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: BackupKind,
					},
					BackupContentType: BackupContentTypePhysical,
				},
				true,
			),
			Entry(
				"Inconsistent backup type 3",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: PhysicalBackupKind,
					},
					BackupContentType: BackupContentTypeLogical,
				},
				true,
			),
			Entry(
				"Inconsistent backup type 4",
				&BootstrapFrom{
					VolumeSnapshotRef: &LocalObjectReference{
						Name: "test",
					},
					BackupContentType: BackupContentTypeLogical,
				},
				true,
			),
			Entry(
				"VolumeSnapshot and S3 mutually exclusive",
				&BootstrapFrom{
					VolumeSnapshotRef: &LocalObjectReference{
						Name: "test",
					},
					S3: &S3{
						Bucket: "test",
					},
				},
				true,
			),
			Entry(
				"VolumeSnapshot and Volume mutually exclusive",
				&BootstrapFrom{
					VolumeSnapshotRef: &LocalObjectReference{
						Name: "test",
					},
					Volume: &StorageVolumeSource{
						PersistentVolumeClaim: &PersistentVolumeClaimVolumeSource{
							ClaimName: "test",
						},
					},
				},
				true,
			),
			Entry(
				"VolumeSnapshot and RestoreJob mutually exclusive",
				&BootstrapFrom{
					VolumeSnapshotRef: &LocalObjectReference{
						Name: "test",
					},
					RestoreJob: &Job{},
				},
				true,
			),
			Entry(
				"Valid 1",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
					},
					BackupContentType: BackupContentTypeLogical,
				},
				false,
			),
			Entry(
				"Valid 2",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: BackupKind,
					},
					BackupContentType: BackupContentTypeLogical,
				},
				false,
			),
			Entry(
				"Valid 3",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: PhysicalBackupKind,
					},
					BackupContentType: BackupContentTypePhysical,
				},
				false,
			),
			Entry(
				"Valid 4",
				&BootstrapFrom{
					VolumeSnapshotRef: &LocalObjectReference{
						Name: "test",
					},
					BackupContentType: BackupContentTypePhysical,
				},
				false,
			),
			Entry(
				"Valid 5",
				&BootstrapFrom{
					S3: &S3{
						Bucket: "test",
					},
					BackupContentType: BackupContentTypePhysical,
				},
				false,
			),
			Entry(
				"Valid 6",
				&BootstrapFrom{
					S3: &S3{
						Bucket: "test",
					},
					BackupContentType: BackupContentTypeLogical,
				},
				false,
			),
			Entry(
				"Valid 7",
				&BootstrapFrom{
					Volume: &StorageVolumeSource{
						NFS: &NFSVolumeSource{
							Server: "nas.local",
						},
					},
					BackupContentType: BackupContentTypeLogical,
				},
				false,
			),
			Entry(
				"Valid 8",
				&BootstrapFrom{
					Volume: &StorageVolumeSource{
						NFS: &NFSVolumeSource{
							Server: "nas.local",
						},
					},
					BackupContentType: BackupContentTypePhysical,
				},
				false,
			),
			Entry(
				"Valid 9",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: PhysicalBackupKind,
					},
					VolumeSnapshotRef: &LocalObjectReference{
						Name: "test",
					},
					BackupContentType: BackupContentTypePhysical,
				},
				false,
			),
		)

		DescribeTable(
			"Should default",
			func(bootstrapFrom *BootstrapFrom, mariadb *MariaDB, expected *BootstrapFrom) {
				bootstrapFrom.SetDefaults(mariadb)
				Expect(bootstrapFrom).To(BeEquivalentTo(expected))
			},
			Entry(
				"Empty",
				&BootstrapFrom{},
				&MariaDB{
					ObjectMeta: objMeta,
				},
				&BootstrapFrom{
					BackupContentType: BackupContentTypeLogical,
				},
			),
			Entry(
				"Logical backup",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
				},
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
					},
					BackupContentType: BackupContentTypeLogical,
				},
			),
			Entry(
				"Logical backup with kind",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: BackupKind,
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
				},
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: BackupKind,
					},
					BackupContentType: BackupContentTypeLogical,
				},
			),
			Entry(
				"Physical backup",
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: PhysicalBackupKind,
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
				},
				&BootstrapFrom{
					BackupRef: &TypedLocalObjectReference{
						Name: "test",
						Kind: PhysicalBackupKind,
					},
					BackupContentType: BackupContentTypePhysical,
				},
			),
			Entry(
				"Volume snapshot",
				&BootstrapFrom{
					VolumeSnapshotRef: &LocalObjectReference{
						Name: "test",
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
				},
				&BootstrapFrom{
					VolumeSnapshotRef: &LocalObjectReference{
						Name: "test",
					},
					BackupContentType: BackupContentTypePhysical,
				},
			),
			Entry(
				"PhysicalBackup in S3",
				&BootstrapFrom{
					S3: &S3{
						Bucket: "test",
					},
					BackupContentType: BackupContentTypePhysical,
				},
				&MariaDB{
					ObjectMeta: objMeta,
				},
				&BootstrapFrom{
					S3: &S3{
						Bucket: "test",
					},
					BackupContentType: BackupContentTypePhysical,
					Volume: &StorageVolumeSource{
						EmptyDir: &EmptyDirVolumeSource{},
					},
				},
			),
			Entry(
				"PhysicalBackup in S3 with staging storage",
				&BootstrapFrom{
					S3: &S3{
						Bucket: "test",
					},
					BackupContentType: BackupContentTypePhysical,
					StagingStorage: &BackupStagingStorage{
						PersistentVolumeClaim: &PersistentVolumeClaimSpec{
							StorageClassName: ptr.To("test"),
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
				},
				&BootstrapFrom{
					S3: &S3{
						Bucket: "test",
					},
					BackupContentType: BackupContentTypePhysical,
					StagingStorage: &BackupStagingStorage{
						PersistentVolumeClaim: &PersistentVolumeClaimSpec{
							StorageClassName: ptr.To("test"),
						},
					},
					Volume: &StorageVolumeSource{
						PersistentVolumeClaim: &PersistentVolumeClaimVolumeSource{
							ClaimName: "mariadb-obj-physicalbackup-staging",
						},
					},
				},
			),
		)
	})
})
