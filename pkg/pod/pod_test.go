package pod

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestPodInitializing(t *testing.T) {
	running := corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	terminated := corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}}
	sidecar := corev1.Container{Name: "proxy", RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways)}
	initC := corev1.Container{Name: "init"}

	tests := []struct {
		name     string
		inits    []corev1.Container
		statuses []corev1.ContainerStatus
		want     bool
	}{
		{"no init containers", nil, nil, false},
		{
			"regular init running",
			[]corev1.Container{initC},
			[]corev1.ContainerStatus{{Name: "init", State: running}},
			true,
		},
		{
			"only sidecar running",
			[]corev1.Container{sidecar, initC},
			[]corev1.ContainerStatus{{Name: "proxy", State: running}, {Name: "init", State: terminated}},
			false,
		},
		{
			"sidecar and regular init running",
			[]corev1.Container{sidecar, initC},
			[]corev1.ContainerStatus{{Name: "proxy", State: running}, {Name: "init", State: running}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Spec:   corev1.PodSpec{InitContainers: tt.inits},
				Status: corev1.PodStatus{InitContainerStatuses: tt.statuses},
			}
			if got := PodInitializing(pod); got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
