package v1alpha1

import (
	v1alpha2 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"time"

	"github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
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
	Context("When creating a v1alpha1.MaxScale", func() {
		meta := metav1.ObjectMeta{
			Name:      "maxscale-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(mxs *v1alpha2.MaxScale, wantErr bool) {
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
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						Servers: []v1alpha2.MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-1.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: v1alpha2.MonitorModuleMariadb,
						},
					},
				},
				true,
			),
			Entry(
				"Invalid server addresses",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						Servers: []v1alpha2.MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
							{
								Name:    "mariadb-1",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: v1alpha2.MonitorModuleMariadb,
						},
					},
				},
				true,
			),
			Entry(
				"No server sources",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: v1alpha2.MonitorModuleMariadb,
						},
					},
				},
				true,
			),
			Entry(
				"Multiple server sources",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						MariaDBRef: &v1alpha2.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb",
							},
						},
						Servers: []v1alpha2.MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: v1alpha2.MonitorModuleMariadb,
						},
					},
				},
				true,
			),
			Entry(
				"No monitor",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						Servers: []v1alpha2.MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
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
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						MariaDBRef: &v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb",
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: "foo",
						},
					},
				},
				true,
			),
			Entry(
				"Invalid service names",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						Servers: []v1alpha2.MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadConnRoute,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3307,
								},
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: v1alpha2.MonitorModuleMariadb,
						},
					},
				},
				true,
			),
			Entry(
				"Invalid service ports",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						Servers: []v1alpha2.MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
							{
								Name:   "conn-router",
								Router: v1alpha2.ServiceRouterReadConnRoute,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: v1alpha2.MonitorModuleMariadb,
						},
					},
				},
				true,
			),
			Entry(
				"Invalid auth generate",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						Servers: []v1alpha2.MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: v1alpha2.MonitorModuleMariadb,
						},
						Auth: v1alpha2.MaxScaleAuth{
							Generate: ptr.To(true),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid PodDisruptionBudget",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						Servers: []v1alpha2.MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: v1alpha2.MonitorModuleMariadb,
						},
						PodDisruptionBudget: &v1alpha1.PodDisruptionBudget{
							MaxUnavailable: ptr.To(intstr.FromString("50%")),
							MinAvailable:   ptr.To(intstr.FromString("50%")),
						},
					},
				},
				true,
			),
			Entry(
				"Valid with MariaDB reference",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						MariaDBRef: &v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						PodDisruptionBudget: &v1alpha1.PodDisruptionBudget{
							MaxUnavailable: ptr.To(intstr.FromString("50%")),
						},
					},
				},
				false,
			),
			Entry(
				"Valid with servers",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						Servers: []v1alpha2.MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []v1alpha2.MaxScaleService{
							{
								Name:   "rw-router",
								Router: v1alpha2.ServiceRouterReadWriteSplit,
								Listener: v1alpha2.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: v1alpha2.MaxScaleMonitor{
							Module: v1alpha2.MonitorModuleMariadb,
						},
						PodDisruptionBudget: &v1alpha1.PodDisruptionBudget{
							MaxUnavailable: ptr.To(intstr.FromString("50%")),
						},
					},
				},
				false,
			),
			Entry(
				"Invalid TLS",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						MariaDBRef: &v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb",
							},
						},
						TLS: &v1alpha2.MaxScaleTLS{
							Enabled: true,
							ListenerCertSecretRef: &v1alpha1.LocalObjectReference{
								Name: "listener-cert",
							},
						},
					},
				},
				true,
			),
			Entry(
				"Valid TLS",
				&v1alpha2.MaxScale{
					ObjectMeta: meta,
					Spec: v1alpha2.MaxScaleSpec{
						MariaDBRef: &v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb",
							},
						},
						TLS: &v1alpha2.MaxScaleTLS{
							Enabled: true,
							ListenerCASecretRef: &v1alpha1.LocalObjectReference{
								Name: "listener-ca",
							},
							ListenerCertSecretRef: &v1alpha1.LocalObjectReference{
								Name: "listener-cert",
							},
						},
					},
				},
				false,
			),
		)
	})

	Context("When updating a v1alpha1.MaxScale", Ordered, func() {
		key := types.NamespacedName{
			Name:      "maxscale-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			mxs := v1alpha2.MaxScale{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha2.MaxScaleSpec{
					MariaDBRef: &v1alpha1.MariaDBRef{
						ObjectReference: v1alpha1.ObjectReference{
							Name: "mariadb",
						},
					},
					Services: []v1alpha2.MaxScaleService{
						{
							Name:   "rw-router",
							Router: v1alpha2.ServiceRouterReadWriteSplit,
							Listener: v1alpha2.MaxScaleListener{
								Port: 3306,
							},
						},
						{
							Name:   "rconn-master-router",
							Router: v1alpha2.ServiceRouterReadConnRoute,
							Listener: v1alpha2.MaxScaleListener{
								Port: 3307,
								Params: map[string]string{
									"router_options": "master",
								},
							},
						},
						{
							Name:   "rconn-slave-router",
							Router: v1alpha2.ServiceRouterReadConnRoute,
							Listener: v1alpha2.MaxScaleListener{
								Port: 3308,
								Params: map[string]string{
									"router_options": "slave",
								},
							},
						},
					},
					Admin: v1alpha2.MaxScaleAdmin{
						Port: 8989,
					},
					Auth: v1alpha2.MaxScaleAuth{
						Generate:      ptr.To(true),
						AdminUsername: "foo",
					},
					KubernetesService: &v1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &v1alpha1.Metadata{
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
			func(patchFn func(mxs *v1alpha2.MaxScale), wantErr bool) {
				var mxs v1alpha2.MaxScale
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
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Image = "mariadb/maxscale:23.07"
				},
				false,
			),
			Entry(
				"Adding Servers",
				func(mxs *v1alpha2.MaxScale) {
					servers := []v1alpha2.MaxScaleServer{
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
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Services = append(mxs.Spec.Services, v1alpha2.MaxScaleService{
						Name:   "rconn-router",
						Router: v1alpha2.ServiceRouterReadConnRoute,
						Listener: v1alpha2.MaxScaleListener{
							Port: 3309,
						}},
					)
				},
				false,
			),
			Entry(
				"Updating Service",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Services[0].Listener.Port = 1111
				},
				true,
			),
			Entry(
				"Updating Monitor interval",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Monitor.Interval = metav1.Duration{Duration: 1 * time.Second}
				},
				false,
			),
			Entry(
				"Updating Monitor module",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Monitor.Module = v1alpha2.MonitorModuleMariadb
				},
				false,
			),
			Entry(
				"Updating Admin",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Admin.Port = 9090
				},
				false,
			),
			Entry(
				"Updating Config",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Config.Params = map[string]string{
						"foo": "bar",
					}
				},
				false,
			),
			Entry(
				"Updating Auth generate",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Auth.Generate = ptr.To(false)
				},
				true,
			),
			Entry(
				"Updating Auth",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Auth.AdminUsername = "bar"
				},
				true,
			),
			Entry(
				"Updating Replicas",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Replicas = 3
				},
				false,
			),
			Entry(
				"Updating Resources",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.Resources = &v1alpha1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("200m"),
						},
					}
				},
				false,
			),
			Entry(
				"Updating to invalid TLS",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.TLS = &v1alpha2.MaxScaleTLS{
						Enabled: true,
						ListenerCertSecretRef: &v1alpha1.LocalObjectReference{
							Name: "server-cert",
						},
					}
				},
				true,
			),
			Entry(
				"Updating to valid TLS",
				func(mxs *v1alpha2.MaxScale) {
					mxs.Spec.TLS = &v1alpha2.MaxScaleTLS{
						Enabled: true,
						ListenerCASecretRef: &v1alpha1.LocalObjectReference{
							Name: "server-ca",
						},
						ListenerCertSecretRef: &v1alpha1.LocalObjectReference{
							Name: "server-cert",
						},
					}
				},
				false,
			),
		)
	})
})
