package statefulset

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestStatefulSetHasChanged(t *testing.T) {
	tests := []struct {
		name     string
		sts      *appsv1.StatefulSet
		otherSts *appsv1.StatefulSet
		wantBool bool
	}{
		{
			name:     "nil",
			sts:      nil,
			otherSts: nil,
			wantBool: true,
		},
		{
			name: "diff template",
			sts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Image: "mariadb:10.6",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"cpu":    resource.MustParse("100m"),
											"memory": resource.MustParse("300Mi"),
										},
										Limits: corev1.ResourceList{
											"memory": resource.MustParse("2Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
			otherSts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Image: "mariadb:10.7",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"cpu":    resource.MustParse("100m"),
											"memory": resource.MustParse("300Mi"),
										},
										Limits: corev1.ResourceList{
											"memory": resource.MustParse("2Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantBool: true,
		},
		{
			name: "diff update strategy",
			sts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.OnDeleteStatefulSetStrategyType,
					},
				},
			},
			otherSts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							MaxUnavailable: ptr.To(intstr.FromInt(1)),
						},
					},
				},
			},
			wantBool: true,
		},
		{
			name: "diff replicas",
			sts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(3)),
				},
			},
			otherSts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(5)),
				},
			},
			wantBool: true,
		},
		{
			name: "same",
			sts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Image: "mariadb:10.6",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"cpu":    resource.MustParse("100m"),
											"memory": resource.MustParse("300Mi"),
										},
										Limits: corev1.ResourceList{
											"memory": resource.MustParse("2Gi"),
										},
									},
								},
							},
						},
					},
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							MaxUnavailable: ptr.To(intstr.FromInt(1)),
						},
					},
					Replicas: ptr.To(int32(3)),
				},
			},
			otherSts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Image: "mariadb:10.6",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"cpu":    resource.MustParse("100m"),
											"memory": resource.MustParse("300Mi"),
										},
										Limits: corev1.ResourceList{
											"memory": resource.MustParse("2Gi"),
										},
									},
								},
							},
						},
					},
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							MaxUnavailable: ptr.To(intstr.FromInt(1)),
						},
					},
					Replicas: ptr.To(int32(3)),
				},
			},
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBool := StatefulSetHasChanged(tt.sts, tt.otherSts)
			if !reflect.DeepEqual(gotBool, tt.wantBool) {
				t.Errorf("expecting StatefulSetHasChanged returned value to be:\n%v\ngot:\n%v\n", tt.wantBool, gotBool)
			}
		})
	}
}
