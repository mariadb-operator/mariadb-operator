package v1alpha1

import (
	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
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
			"Is initial user enabled?",
			func(mdb *MariaDB, wantIsEnabled bool) {
				isEnabled := mdb.IsInitialUserEnabled()
				Expect(isEnabled).To(Equal(wantIsEnabled))
			},
			Entry(
				"Empty",
				&MariaDB{
					Spec: MariaDBSpec{},
				},
				false,
			),
			Entry(
				"Username and Database",
				&MariaDB{
					Spec: MariaDBSpec{
						Username: ptr.To("test"),
						Database: ptr.To("test"),
					},
				},
				false,
			),
			Entry(
				"Username and Password",
				&MariaDB{
					Spec: MariaDBSpec{
						PasswordSecretKeyRef: &GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "test",
								},
								Key: "test",
							},
						},
						Username: ptr.To("test"),
					},
				},
				false,
			),
			Entry(
				"Username, Password and Database",
				&MariaDB{
					Spec: MariaDBSpec{
						PasswordSecretKeyRef: &GeneratedSecretKeyRef{
							SecretKeySelector: SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "test",
								},
								Key: "test",
							},
						},
						Username: ptr.To("test"),
						Database: ptr.To("test"),
					},
				},
				true,
			),
			Entry(
				"Username, Password hash and Database",
				&MariaDB{
					Spec: MariaDBSpec{
						PasswordHashSecretKeyRef: &SecretKeySelector{
							LocalObjectReference: LocalObjectReference{
								Name: "test",
							},
							Key: "test",
						},
						Username: ptr.To("test"),
						Database: ptr.To("test"),
					},
				},
				true,
			),
			Entry(
				"Username, Password plugin and Database",
				&MariaDB{
					Spec: MariaDBSpec{
						PasswordPlugin: &PasswordPlugin{
							PluginNameSecretKeyRef: &SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "test",
								},
								Key: "test",
							},
							PluginArgSecretKeyRef: &SecretKeySelector{
								LocalObjectReference: LocalObjectReference{
									Name: "test",
								},
								Key: "test",
							},
						},
						Username: ptr.To("test"),
						Database: ptr.To("test"),
					},
				},
				true,
			),
		)

		DescribeTable(
			"Is Galera enterprise TLS enabled?",
			func(mdb *MariaDB, isEnterprise bool, defaultMariadbVersion string, wantIsEnabled, wantErr bool) {
				discovery, err := discovery.NewFakeDiscovery(isEnterprise)
				Expect(err).ToNot(HaveOccurred())
				logger := logr.Discard()

				isEnabled, err := mdb.IsGaleraEnterpriseTLSAvailable(discovery, defaultMariadbVersion, logger)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
				Expect(isEnabled).To(Equal(wantIsEnabled))
			},
			Entry(
				"Standalone",
				&MariaDB{
					Spec: MariaDBSpec{
						Image: "mariadb:11.4.3",
					},
				},
				false,
				"",
				false,
				false,
			),
			Entry(
				"Galera",
				&MariaDB{
					Spec: MariaDBSpec{
						Image: "mariadb:11.4.3",
						Galera: &Galera{
							Enabled: true,
						},
					},
				},
				false,
				"",
				false,
				false,
			),
			Entry(
				"Galera TLS",
				&MariaDB{
					Spec: MariaDBSpec{
						Image: "mariadb:11.4.3",
						Galera: &Galera{
							Enabled: true,
						},
						TLS: &TLS{
							Enabled: true,
						},
					},
				},
				false,
				"",
				false,
				false,
			),
			Entry(
				"Enabled",
				&MariaDB{
					Spec: MariaDBSpec{
						Image: "docker.mariadb.com/enterprise-server:10.6",
						Galera: &Galera{
							Enabled: true,
						},
						TLS: &TLS{
							Enabled: true,
						},
					},
				},
				true,
				"",
				true,
				false,
			),
			Entry(
				"Unsupported version",
				&MariaDB{
					Spec: MariaDBSpec{
						Image: "docker.mariadb.com/enterprise-server:10.5",
						Galera: &Galera{
							Enabled: true,
						},
						TLS: &TLS{
							Enabled: true,
						},
					},
				},
				true,
				"",
				false,
				false,
			),
			Entry(
				"Invalid image",
				&MariaDB{
					Spec: MariaDBSpec{
						Image: "docker.mariadb.com/enterprise-server@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
						Galera: &Galera{
							Enabled: true,
						},
						TLS: &TLS{
							Enabled: true,
						},
					},
				},
				true,
				"",
				false,
				true,
			),
			Entry(
				"Invalid image with default",
				&MariaDB{
					Spec: MariaDBSpec{
						Image: "docker.mariadb.com/enterprise-server:@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
						Galera: &Galera{
							Enabled: true,
						},
						TLS: &TLS{
							Enabled: true,
						},
					},
				},
				true,
				"10.6",
				true,
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
})
