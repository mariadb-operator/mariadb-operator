package statefulset

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStatefulSetValidPodName(t *testing.T) {
	tests := []struct {
		name     string
		meta     metav1.ObjectMeta
		replicas int
		podName  string
		wantErr  bool
	}{
		{
			name: "empty",
			meta: metav1.ObjectMeta{
				Name: "",
			},
			replicas: 0,
			podName:  "",
			wantErr:  true,
		},
		{
			name: "negative replicas",
			meta: metav1.ObjectMeta{
				Name: "",
			},
			replicas: -1,
			podName:  "",
			wantErr:  true,
		},
		{
			name: "no index no prefix",
			meta: metav1.ObjectMeta{
				Name: "mariadb-galera",
			},
			replicas: 3,
			podName:  "foo",
			wantErr:  true,
		},
		{
			name: "no index",
			meta: metav1.ObjectMeta{
				Name: "mariadb-galera",
			},
			replicas: 3,
			podName:  "mariadb-galera",
			wantErr:  true,
		},
		{
			name: "invalid index",
			meta: metav1.ObjectMeta{
				Name: "mariadb-galera",
			},
			replicas: 3,
			podName:  "mariadb-galera-5",
			wantErr:  true,
		},
		{
			name: "no prefix",
			meta: metav1.ObjectMeta{
				Name: "mariadb-galera",
			},
			replicas: 3,
			podName:  "foo-0",
			wantErr:  true,
		},
		{
			name: "valid",
			meta: metav1.ObjectMeta{
				Name: "mariadb-galera",
			},
			replicas: 3,
			podName:  "mariadb-galera-0",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidPodName(tt.meta, tt.replicas, tt.podName)
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err == nil {
				t.Error("expecting error, got nil")
			}
		})
	}
}
