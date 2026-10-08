package config

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("threads", func() {
	DescribeTable("computing threads",
		func(mxs *mariadbv1alpha1.MaxScale, wantString string) {
			gotString := threads(mxs)
			Expect(gotString).To(Equal(wantString))
		},
		Entry("cpu limit defined",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu": resource.MustParse("1"),
							},
						},
					},
				},
			},
			"1",
		),
		Entry("cpu limit defined round up",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu": resource.MustParse("200m"),
							},
						},
					},
				},
			},
			"1",
		),
		Entry("resources not defined",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{},
			},
			"auto",
		),
		Entry("only requests defined",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("200m"),
							},
						},
					},
				},
			},
			"auto",
		),
		Entry("other limit defined",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"memory": resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
			"auto",
		),
	)
})

var _ = Describe("queryClassifierCacheSize", func() {
	DescribeTable("computing query classifier cache size",
		func(mxs *mariadbv1alpha1.MaxScale, wantString string) {
			gotString := queryClassifierCacheSize(mxs)
			Expect(gotString).To(Equal(wantString))
		},
		Entry("memory limit defined",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"memory": resource.MustParse("1G"),
							},
						},
					},
				},
			},
			"150000000",
		),
		Entry("resources not defined",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{},
			},
			"",
		),
		Entry("only requests defined",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"memory": resource.MustParse("2Gi"),
							},
						},
					},
				},
			},
			"",
		),
		Entry("other limit defined",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
					},
				},
			},
			"",
		),
	)
})
