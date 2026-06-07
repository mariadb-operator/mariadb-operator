package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ServiceMeta", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "service",
	}

	DescribeTable("should build the Service metadata",
		func(opts ServiceOpts, wantMeta *mariadbv1alpha1.Metadata) {
			svc, err := builder.BuildService(key, &mariadbv1alpha1.MariaDB{}, opts)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&svc.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			ServiceOpts{
				ExtraMeta:             &mariadbv1alpha1.Metadata{},
				ExcludeSelectorLabels: true,
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("meta",
			ServiceOpts{
				ServiceTemplate: mariadbv1alpha1.ServiceTemplate{
					Metadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"metallb.io/loadBalancerIPs": "172.18.0.20",
						},
					},
				},
				ExcludeSelectorLabels: true,
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"metallb.io/loadBalancerIPs": "172.18.0.20",
				},
			},
		),
		Entry("extra meta",
			ServiceOpts{
				ExtraMeta: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
				ExcludeSelectorLabels: true,
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("meta and extra meta",
			ServiceOpts{
				ServiceTemplate: mariadbv1alpha1.ServiceTemplate{
					Metadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"metallb.io/loadBalancerIPs": "172.18.0.20",
						},
					},
				},
				ExtraMeta: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
				ExcludeSelectorLabels: true,
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io":          "mariadb",
					"metallb.io/loadBalancerIPs": "172.18.0.20",
				},
			},
		),
	)
})

var _ = Describe("ServicePorts", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "service",
	}

	DescribeTable("should return an error building the Service",
		func(opts ServiceOpts) {
			_, err := builder.BuildService(key, &mariadbv1alpha1.MariaDB{}, opts)
			Expect(err).To(HaveOccurred())
		},
		Entry("duplicated port names",
			ServiceOpts{
				Ports: []corev1.ServicePort{
					{
						Name: "mariadb",
						Port: 3306,
					},
					{
						Name: "mariadb",
						Port: 9995,
					},
				},
				ExcludeSelectorLabels: true,
			},
		),
		Entry("duplicated port numbers",
			ServiceOpts{
				Ports: []corev1.ServicePort{
					{
						Name: "mariadb",
						Port: 3306,
					},
					{
						Name: "disk-usage-exporter",
						Port: 3306,
					},
				},
				ExcludeSelectorLabels: true,
			},
		),
	)
})

var _ = Describe("ServiceLoadBalancerClass", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "service",
	}
	loadBalancerClass := "tailscale"

	DescribeTable("should build the Service LoadBalancerClass",
		func(opts ServiceOpts, wantLoadBalancerClass *string) {
			svc, err := builder.BuildService(key, &mariadbv1alpha1.MariaDB{}, opts)
			Expect(err).NotTo(HaveOccurred())
			if wantLoadBalancerClass == nil {
				Expect(svc.Spec.LoadBalancerClass).To(BeNil())
			} else {
				Expect(svc.Spec.LoadBalancerClass).NotTo(BeNil())
				Expect(*svc.Spec.LoadBalancerClass).To(Equal(*wantLoadBalancerClass))
			}
		},
		Entry("no loadBalancerClass",
			ServiceOpts{
				ExcludeSelectorLabels: true,
			},
			nil,
		),
		Entry("with loadBalancerClass",
			ServiceOpts{
				ServiceTemplate: mariadbv1alpha1.ServiceTemplate{
					LoadBalancerClass: &loadBalancerClass,
				},
				ExcludeSelectorLabels: true,
			},
			&loadBalancerClass,
		),
	)
})
