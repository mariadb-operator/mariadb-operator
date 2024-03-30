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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MaxScale webhook", func() {
	Context("When creating a MaxScale", func() {
		meta := metav1.ObjectMeta{
			Name:      "maxscale-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(mxs *MaxScale, wantErr bool) {
				_ = k8sClient.Delete(testCtx, mxs)
				err := k8sClient.Create(testCtx, mxs)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Invalid server names",
				&MaxScale{
					ObjectMeta: meta,
					Spec: MaxScaleSpec{
						Servers: []MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-1.mariadb-repl-internal.default.svc.cluster.local",
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
				true,
			),
			Entry(
				"Invalid server addresses",
				&MaxScale{
					ObjectMeta: meta,
					Spec: MaxScaleSpec{
						Servers: []MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
							{
								Name:    "mariadb-1",
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
				true,
			),
			Entry(
				"No server sources",
				&MaxScale{
					ObjectMeta: meta,
					Spec: MaxScaleSpec{
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
				true,
			),
			Entry(
				"Multiple server sources",
				&MaxScale{
					ObjectMeta: meta,
					Spec: MaxScaleSpec{
						MariaDBRef: &MariaDBRef{
							ObjectReference: corev1.ObjectReference{
								Name: "mariadb",
							},
						},
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
				true,
			),
			Entry(
				"No monitor",
				&MaxScale{
					ObjectMeta: meta,
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
					},
				},
				true,
			),
			Entry(
				"Invalid monitor",
				&MaxScale{
					ObjectMeta: meta,
					Spec: MaxScaleSpec{
						MariaDBRef: &MariaDBRef{
							ObjectReference: corev1.ObjectReference{
								Name: "mariadb",
							},
						},
						Monitor: MaxScaleMonitor{
							Module: "foo",
						},
					},
				},
				true,
			),
			Entry(
				"Invalid service names",
				&MaxScale{
					ObjectMeta: meta,
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
							{
								Name:   "rw-router",
								Router: ServiceRouterReadConnRoute,
								Listener: MaxScaleListener{
									Port: 3307,
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Module: MonitorModuleMariadb,
						},
					},
				},
				true,
			),
			Entry(
				"Invalid service ports",
				&MaxScale{
					ObjectMeta: meta,
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
							{
								Name:   "conn-router",
								Router: ServiceRouterReadConnRoute,
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
				true,
			),
			Entry(
				"Invalid auth generate",
				&MaxScale{
					ObjectMeta: meta,
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
						Auth: MaxScaleAuth{
							Generate: ptr.To(true),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid PodDisruptionBudget",
				&MaxScale{
					ObjectMeta: meta,
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
						PodDisruptionBudget: &PodDisruptionBudget{
							MaxUnavailable: func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
							MinAvailable:   func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
						},
					},
				},
				true,
			),
			Entry(
				"Valid with MariaDB reference",
				&MaxScale{
					ObjectMeta: meta,
					Spec: MaxScaleSpec{
						MariaDBRef: &MariaDBRef{
							ObjectReference: corev1.ObjectReference{
								Name: "mariadb",
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
						PodDisruptionBudget: &PodDisruptionBudget{
							MaxUnavailable: func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
						},
					},
				},
				false,
			),
			Entry(
				"Valid with servers",
				&MaxScale{
					ObjectMeta: meta,
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
						PodDisruptionBudget: &PodDisruptionBudget{
							MaxUnavailable: func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
						},
					},
				},
				false,
			),
		)
	})

	Context("When updating a MaxScale", Ordered, func() {
		key := types.NamespacedName{
			Name:      "maxscale-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			mxs := MaxScale{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: MaxScaleSpec{
					MariaDBRef: &MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: "mariadb",
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
					Admin: MaxScaleAdmin{
						Port: 8989,
					},
					Auth: MaxScaleAuth{
						Generate:      ptr.To(true),
						AdminUsername: "foo",
					},
					KubernetesService: &ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": "172.18.0.214",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mxs)).To(Succeed())
		})
		DescribeTable(
			"Should validate",
			func(patchFn func(mxs *MaxScale), wantErr bool) {
				var mxs MaxScale
				Expect(k8sClient.Get(testCtx, key, &mxs)).To(Succeed())

				patch := client.MergeFrom(mxs.DeepCopy())
				patchFn(&mxs)

				err := k8sClient.Patch(testCtx, &mxs, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating Image",
				func(mxs *MaxScale) {
					mxs.Spec.Image = "mariadb/maxscale:23.07"
				},
				false,
			),
			Entry(
				"Adding Servers",
				func(mxs *MaxScale) {
					servers := []MaxScaleServer{
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
					}
					mxs.Spec.Servers = append(mxs.Spec.Servers, servers...)
				},
				false,
			),
			Entry(
				"Adding Service",
				func(mxs *MaxScale) {
					mxs.Spec.Services = append(mxs.Spec.Services, MaxScaleService{
						Name:   "rconn-router",
						Router: ServiceRouterReadConnRoute,
						Listener: MaxScaleListener{
							Port: 3309,
						}},
					)
				},
				false,
			),
			Entry(
				"Updating Service",
				func(mxs *MaxScale) {
					mxs.Spec.Services[0].Listener.Port = 1111
				},
				true,
			),
			Entry(
				"Updating Monitor interval",
				func(mxs *MaxScale) {
					mxs.Spec.Monitor.Interval = metav1.Duration{Duration: 1 * time.Second}
				},
				false,
			),
			Entry(
				"Updating Monitor module",
				func(mxs *MaxScale) {
					mxs.Spec.Monitor.Module = MonitorModuleMariadb
				},
				false,
			),
			Entry(
				"Updating Admin",
				func(mxs *MaxScale) {
					mxs.Spec.Admin.Port = 9090
				},
				false,
			),
			Entry(
				"Updating Config",
				func(mxs *MaxScale) {
					mxs.Spec.Config.Params = map[string]string{
						"foo": "bar",
					}
				},
				false,
			),
			Entry(
				"Updating Auth generate",
				func(mxs *MaxScale) {
					mxs.Spec.Auth.Generate = ptr.To(false)
				},
				true,
			),
			Entry(
				"Updating Auth",
				func(mxs *MaxScale) {
					mxs.Spec.Auth.AdminUsername = "bar"
				},
				true,
			),
			Entry(
				"Updating Replicas",
				func(mxs *MaxScale) {
					mxs.Spec.Replicas = 3
				},
				false,
			),
			Entry(
				"Updating Resources",
				func(mxs *MaxScale) {
					mxs.Spec.Resources = &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("200m"),
						},
					}
				},
				false,
			),
		)
	})
})
