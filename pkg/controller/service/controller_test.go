package service

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("updateServicePorts", func() {
	DescribeTable("updates the existing service ports",
		func(existingSvc, desiredSvc, expectedSvc *corev1.Service) {
			updateServicePorts(existingSvc, desiredSvc)
			if expectedSvc == nil {
				Expect(existingSvc).To(BeNil())
			} else {
				Expect(existingSvc).To(Equal(expectedSvc))
			}
		},
		Entry("Nil services", nil, nil, nil),
		Entry("Existing service has no ports",
			&corev1.Service{},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
		),
		Entry("Existing service has ports with overlapping ports",
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
		),
		Entry("Desired service has NodePort type and existing port has non-zero NodePort",
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
							NodePort: 30000,
						},
					},
				},
			},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
							NodePort: 30000,
						},
					},
				},
			},
		),
		Entry("Desired service has ClusterIP type and existing port has non-zero NodePort",
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
							NodePort: 30000,
						},
						{
							Port:     80,
							Protocol: corev1.ProtocolTCP,
							NodePort: 30001,
						},
					},
				},
			},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
						},
						{
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
		),
		Entry("Desired service has LoadBalancer type and existing port has non-zero NodePort",
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
							NodePort: 30000,
						},
						{
							Port:     80,
							Protocol: corev1.ProtocolTCP,
							NodePort: 30001,
						},
					},
				},
			},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			&corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     3306,
							Protocol: corev1.ProtocolTCP,
							NodePort: 30000,
						},
						{
							Port:     80,
							Protocol: corev1.ProtocolTCP,
							NodePort: 30001,
						},
					},
				},
			},
		),
	)
})
