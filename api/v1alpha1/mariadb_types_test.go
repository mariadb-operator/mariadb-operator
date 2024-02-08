package v1alpha1

import (
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariaDB types", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-obj",
		Namespace: "mariadb-obj",
	}
	env := &environment.Environment{
		RelatedMariadbImage: "mariadb:11.0.3",
	}
	Context("When creating a MariaDB object", func() {
		DescribeTable(
			"Should default",
			func(mdb, expected *MariaDB, env *environment.Environment) {
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
						EphemeralStorage:  ptr.To(false),
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-obj-root",
							},
							Key: "password",
						},
						Port: 3306,
					},
				},
				env,
			),
			Entry(
				"Image, root password and port",
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:            "mariadb:lts",
						EphemeralStorage: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "root",
							},
							Key: "pwd",
						},
						Port: 3307,
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             "mariadb:lts",
						EphemeralStorage:  ptr.To(false),
						RootEmptyPassword: ptr.To(false),
						RootPasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "root",
							},
							Key: "pwd",
						},
						Port: 3307,
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
						EphemeralStorage:  ptr.To(false),
						RootEmptyPassword: ptr.To(true),
						Port:              3307,
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             "mariadb:lts",
						EphemeralStorage:  ptr.To(false),
						RootEmptyPassword: ptr.To(true),
						Port:              3307,
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
						EphemeralStorage:  ptr.To(false),
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
						EphemeralStorage:  ptr.To(false),
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
						EphemeralStorage:  ptr.To(false),
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
						EphemeralStorage:  ptr.To(false),
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
						EphemeralStorage:  ptr.To(false),
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
						EphemeralStorage:  ptr.To(false),
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
					},
				},
				&MariaDB{
					ObjectMeta: objMeta,
					Spec: MariaDBSpec{
						Image:             "mariadb:lts",
						EphemeralStorage:  ptr.To(false),
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
					},
				},
				env,
			),
		)
	})
})
