package v1alpha1

import (
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("MaxScale types", func() {
	format.MaxLength = 10000
	storageClassName := "test-sc"
	objMeta := metav1.ObjectMeta{
		Name:      "maxscale-obj",
		Namespace: "maxscale-obj",
	}
	env := &environment.OperatorEnv{
		RelatedMariadbImage:          "mariadb/maxscale:23.08",
		RelatedExporterMaxscaleImage: "mariadb/maxscale-prometheus-exporter-ubi:latest",
	}
	mariadbObjMeta := metav1.ObjectMeta{
		Name:      "mdb-maxscale-obj",
		Namespace: "mdb-maxscale-obj",
	}
	mariadb := &MariaDB{
		ObjectMeta: mariadbObjMeta,
	}
	Context("When creating a MaxScale object", func() {
		DescribeTable(
			"Should default",
			func(mxs, expected *MaxScale, env *environment.OperatorEnv) {
				mxs.SetDefaults(env, mariadb)
				Expect(mxs).To(BeEquivalentTo(expected))
			},
			Entry(
				"Single replica",
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						Servers: []MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Module: MonitorModuleMariadb,
						},
					},
				},
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image: env.RelatedMaxscaleImage,
						Servers: []MaxScaleServer{
							{
								Name:     "mariadb-0",
								Address:  "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
						},
						RequeueInterval: &metav1.Duration{Duration: 10 * time.Second},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Name:     "rw-router-listener",
									Port:     3306,
									Protocol: "MariaDBProtocol",
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Name:     "mariadbmon-monitor",
							Module:   MonitorModuleMariadb,
							Interval: metav1.Duration{Duration: 2 * time.Second},
						},
						Admin: MaxScaleAdmin{
							Port:       8989,
							GuiEnabled: ptr.To(true),
						},
						Config: MaxScaleConfig{
							VolumeClaimTemplate: VolumeClaimTemplate{
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
						Auth: MaxScaleAuth{
							AdminUsername: "mariadb-operator",
							AdminPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-admin",
									},
									Key: "password",
								},
								Generate: true,
							},
							DeleteDefaultAdmin: ptr.To(true),
							ClientUsername:     "maxscale-obj-client",
							ClientPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-client",
									},
									Key: "password",
								},
								Generate: true,
							},
							ClientMaxConnections: 30,
							ServerUsername:       "maxscale-obj-server",
							ServerPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-server",
									},
									Key: "password",
								},
								Generate: true,
							},
							ServerMaxConnections: 30,
							MonitorUsername:      "maxscale-obj-monitor",
							MonitorPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-monitor",
									},
									Key: "password",
								},
								Generate: true,
							},
							MonitorMaxConnections: 30,
						},
					},
				},
				env,
			),
			Entry(
				"Custom config volumeClaimTemplate",
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						Servers: []MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Module: MonitorModuleMariadb,
						},
						Config: MaxScaleConfig{
							VolumeClaimTemplate: VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									StorageClassName: &storageClassName,
								},
							},
						},
					},
				},
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image: env.RelatedMaxscaleImage,
						Servers: []MaxScaleServer{
							{
								Name:     "mariadb-0",
								Address:  "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
						},
						RequeueInterval: &metav1.Duration{Duration: 10 * time.Second},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Name:     "rw-router-listener",
									Port:     3306,
									Protocol: "MariaDBProtocol",
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Name:     "mariadbmon-monitor",
							Module:   MonitorModuleMariadb,
							Interval: metav1.Duration{Duration: 2 * time.Second},
						},
						Admin: MaxScaleAdmin{
							Port:       8989,
							GuiEnabled: ptr.To(true),
						},
						Config: MaxScaleConfig{
							VolumeClaimTemplate: VolumeClaimTemplate{
								PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
									StorageClassName: &storageClassName,
								},
							},
						},
						Auth: MaxScaleAuth{
							AdminUsername: "mariadb-operator",
							AdminPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-admin",
									},
									Key: "password",
								},
								Generate: true,
							},
							DeleteDefaultAdmin: ptr.To(true),
							ClientUsername:     "maxscale-obj-client",
							ClientPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-client",
									},
									Key: "password",
								},
								Generate: true,
							},
							ClientMaxConnections: 30,
							ServerUsername:       "maxscale-obj-server",
							ServerPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-server",
									},
									Key: "password",
								},
								Generate: true,
							},
							ServerMaxConnections: 30,
							MonitorUsername:      "maxscale-obj-monitor",
							MonitorPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-monitor",
									},
									Key: "password",
								},
								Generate: true,
							},
							MonitorMaxConnections: 30,
						},
					},
				},
				env,
			),
			Entry(
				"HA",
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						PodTemplate: PodTemplate{
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
							},
						},
						Replicas: 3,
						Servers: []MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
							{
								Name:    "mariadb-1",
								Address: "mariadb-repl-1.mariadb-repl-internal.default.svc.cluster.local",
							},
							{
								Name:    "mariadb-2",
								Address: "mariadb-repl-2.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Port: 3306,
								},
							},
							{
								Name:   "rconn-master-router",
								Router: ServiceRouterReadConnRoute,
								Listener: MaxScaleListener{
									Port: 3307,
									Params: map[string]string{
										"router_options": "master",
									},
								},
							},
							{
								Name:   "rconn-slave-router",
								Router: ServiceRouterReadConnRoute,
								Listener: MaxScaleListener{
									Port: 3308,
									Params: map[string]string{
										"router_options": "slave",
									},
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Module: MonitorModuleMariadb,
						},
						Metrics: &MaxScaleMetrics{
							Enabled: true,
							Exporter: Exporter{
								PodTemplate: PodTemplate{
									Affinity: &AffinityConfig{
										AntiAffinityEnabled: ptr.To(true),
									},
								},
							},
						},
					},
				},
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						PodTemplate: PodTemplate{
							ServiceAccountName: &objMeta.Name,
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
																objMeta.Name,
																mariadbObjMeta.Name,
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
						Image:           env.RelatedMaxscaleImage,
						Replicas:        3,
						RequeueInterval: &metav1.Duration{Duration: 10 * time.Second},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Name:     "rw-router-listener",
									Port:     3306,
									Protocol: "MariaDBProtocol",
								},
							},
							{
								Name:   "rconn-master-router",
								Router: ServiceRouterReadConnRoute,
								Listener: MaxScaleListener{
									Name:     "rconn-master-router-listener",
									Port:     3307,
									Protocol: "MariaDBProtocol",
									Params: map[string]string{
										"router_options": "master",
									},
								},
							},
							{
								Name:   "rconn-slave-router",
								Router: ServiceRouterReadConnRoute,
								Listener: MaxScaleListener{
									Name:     "rconn-slave-router-listener",
									Port:     3308,
									Protocol: "MariaDBProtocol",
									Params: map[string]string{
										"router_options": "slave",
									},
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Name:                  "mariadbmon-monitor",
							Module:                MonitorModuleMariadb,
							Interval:              metav1.Duration{Duration: 2 * time.Second},
							CooperativeMonitoring: ptr.To(CooperativeMonitoringMajorityOfAll),
						},
						Admin: MaxScaleAdmin{
							Port:       8989,
							GuiEnabled: ptr.To(true),
						},
						Config: MaxScaleConfig{
							VolumeClaimTemplate: VolumeClaimTemplate{
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
							Sync: &MaxScaleConfigSync{
								Database: "mysql",
								Interval: metav1.Duration{Duration: 5 * time.Second},
								Timeout:  metav1.Duration{Duration: 10 * time.Second},
							},
						},
						Auth: MaxScaleAuth{
							AdminUsername: "mariadb-operator",
							AdminPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-admin",
									},
									Key: "password",
								},
								Generate: true,
							},
							DeleteDefaultAdmin: ptr.To(true),
							MetricsUsername:    "metrics",
							MetricsPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-metrics",
									},
									Key: "password",
								},
								Generate: true,
							},
							ClientUsername: "maxscale-obj-client",
							ClientPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-client",
									},
									Key: "password",
								},
								Generate: true,
							},
							ClientMaxConnections: 90,
							ServerUsername:       "maxscale-obj-server",
							ServerPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-server",
									},
									Key: "password",
								},
								Generate: true,
							},
							ServerMaxConnections: 90,
							MonitorUsername:      "maxscale-obj-monitor",
							MonitorPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-monitor",
									},
									Key: "password",
								},
								Generate: true,
							},
							MonitorMaxConnections: 90,
							SyncUsername:          ptr.To("maxscale-obj-sync"),
							SyncPasswordSecretKeyRef: &GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "maxscale-obj-sync",
									},
									Key: "password",
								},
								Generate: true,
							},
							SyncMaxConnections: ptr.To(int32(90)),
						},
						Servers: []MaxScaleServer{
							{
								Name:     "mariadb-0",
								Address:  "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
							{
								Name:     "mariadb-1",
								Address:  "mariadb-repl-1.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
							{
								Name:     "mariadb-2",
								Address:  "mariadb-repl-2.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
						},
						Metrics: &MaxScaleMetrics{
							Enabled: true,
							Exporter: Exporter{
								Image: "mariadb/maxscale-prometheus-exporter-ubi:latest",
								Port:  9105,
								PodTemplate: PodTemplate{
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
																		objMeta.Name,
																		mariadbObjMeta.Name,
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
					},
				},
				env,
			),
		)
	})
})
