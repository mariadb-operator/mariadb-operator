package v1alpha1

import (
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariaDB types", func() {
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
				mdb.SetDefaults(env)
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
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
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
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "root",
							},
							Key: "pwd",
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
						Image:             "mariadb:lts",
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "root",
							},
							Key: "pwd",
						},
						Port: 3307,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
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
						Image:             "mariadb:lts",
						RootEmptyPassword: ptr.To(true),
						Port:              3307,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
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
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
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
						MyCnfConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-config",
							},
							Key: "my.cnf",
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
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
						MyCnfConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-config",
							},
							Key: "mariadb.cnf",
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
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
						MyCnfConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-config",
							},
							Key: "mariadb.cnf",
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
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
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port:     3306,
						Username: ptr.To("user"),
						Database: ptr.To("test"),
						PasswordSecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-password",
							},
							Key: "password",
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
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
						PasswordSecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "user-password",
							},
							Key: "pwd",
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port:     3306,
						Username: ptr.To("user"),
						Database: ptr.To("test"),
						PasswordSecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "user-password",
							},
							Key: "pwd",
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
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
						Metrics: &Metrics{
							Enabled: true,
						},
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port: 3306,
						Metrics: &Metrics{
							Enabled: true,
							Exporter: Exporter{
								Image: env.RelatedExporterImage,
								Port:  9104,
							},
							Username: "mariadb-obj-metrics",
							PasswordSecretKeyRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb-obj-metrics-password",
								},
								Key: "password",
							},
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
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
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port: 3306,
						MaxScaleRef: &corev1.ObjectReference{
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
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
							Size:                ptr.To(resource.MustParse("100Mi")),
							StorageClassName:    "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
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
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:          ptr.To(false),
							ResizeInUseVolumes: ptr.To(true),
							Size:               ptr.To(resource.MustParse("100Mi")),
							StorageClassName:   "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
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
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
							Size:                ptr.To(resource.MustParse("100Mi")),
							StorageClassName:    "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
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
						Image:             env.RelatedMariadbImage,
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port: 3306,
						Storage: Storage{
							Ephemeral: ptr.To(true),
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
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "root",
							},
							Key: "pwd",
						},
						Port:     3307,
						Username: ptr.To("user"),
						Database: ptr.To("test"),
						PasswordSecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "user-password",
							},
							Key: "pwd",
						},
						MyCnf: ptr.To(`
						[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M
						`),
						MyCnfConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-config",
							},
							Key: "mariadb.cnf",
						},
						Metrics: &Metrics{
							Enabled: true,
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
							Size:                ptr.To(resource.MustParse("100Mi")),
							StorageClassName:    "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
									StorageClassName: ptr.To("my-class"),
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
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "root",
							},
							Key: "pwd",
						},
						Port:     3307,
						Username: ptr.To("user"),
						Database: ptr.To("test"),
						PasswordSecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "user-password",
							},
							Key: "pwd",
						},
						MyCnf: ptr.To(`
						[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M
						`),
						MyCnfConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-config",
							},
							Key: "mariadb.cnf",
						},
						Metrics: &Metrics{
							Enabled: true,
							Exporter: Exporter{
								Image: env.RelatedExporterImage,
								Port:  9104,
							},
							Username: "mariadb-obj-metrics",
							PasswordSecretKeyRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb-obj-metrics-password",
								},
								Key: "password",
							},
						},
						Storage: Storage{
							Ephemeral:           ptr.To(false),
							ResizeInUseVolumes:  ptr.To(true),
							WaitForVolumeResize: ptr.To(true),
							Size:                ptr.To(resource.MustParse("100Mi")),
							StorageClassName:    "my-class",
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("100Mi"),
										},
									},
									StorageClassName: ptr.To("my-class"),
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
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
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
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
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
				"Storage size mistmatch",
				&MariaDB{
					Spec: MariaDBSpec{
						Storage: Storage{
							Size: ptr.To(resource.MustParse("50Mi")),
							VolumeClaimTemplate: &VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
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
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
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
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
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
