package config

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestThreads(t *testing.T) {
	tests := []struct {
		name       string
		mxs        *mariadbv1alpha1.MaxScale
		wantString string
	}{
		{
			name: "cpu limit defined",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu": resource.MustParse("1"),
							},
						},
					},
				},
			},
			wantString: "1",
		},
		{
			name: "cpu limit defined round up",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu": resource.MustParse("200m"),
							},
						},
					},
				},
			},
			wantString: "1",
		},
		{
			name: "resources not defined",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{},
			},
			wantString: "auto",
		},
		{
			name: "only requests defined",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("200m"),
							},
						},
					},
				},
			},
			wantString: "auto",
		},
		{
			name: "other limit defined",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"memory": resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
			wantString: "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotString := threads(tt.mxs)
			if tt.wantString != gotString {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantString, gotString)
			}
		})
	}
}

func TestQueryClassifierCacheSize(t *testing.T) {
	tests := []struct {
		name       string
		mxs        *mariadbv1alpha1.MaxScale
		wantString string
	}{
		{
			name: "memory limit defined",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"memory": resource.MustParse("1G"),
							},
						},
					},
				},
			},
			wantString: "150000000",
		},
		{
			name: "resources not defined",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{},
			},
			wantString: "",
		},
		{
			name: "only requests defined",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"memory": resource.MustParse("2Gi"),
							},
						},
					},
				},
			},
			wantString: "",
		},
		{
			name: "other limit defined",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
					},
				},
			},
			wantString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotString := queryClassifierCacheSize(tt.mxs)
			if tt.wantString != gotString {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantString, gotString)
			}
		})
	}
}
