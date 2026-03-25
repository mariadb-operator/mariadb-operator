package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
)

func TestIsReplicaBootstrapScaleOutRecovery(t *testing.T) {
	testCases := map[string]struct {
		annotations map[string]string
		fromIndex   int
		pvcUIDs     map[int]string
		want        bool
	}{
		"lost tail replica pvc recreated": {
			annotations: map[string]string{
				storagePVCUIDAnnotationKey(2): "old-tail-uid",
			},
			fromIndex: 2,
			pvcUIDs: map[int]string{
				2: "new-tail-uid",
			},
			want: true,
		},
		"new replica from user scale out": {
			annotations: map[string]string{
				storagePVCUIDAnnotationKey(0): "primary-uid",
				storagePVCUIDAnnotationKey(1): "replica-uid",
			},
			fromIndex: 2,
			pvcUIDs: map[int]string{
				2: "new-tail-uid",
			},
			want: false,
		},
		"tracked tail pvc unchanged": {
			annotations: map[string]string{
				storagePVCUIDAnnotationKey(2): "tail-uid",
			},
			fromIndex: 2,
			pvcUIDs: map[int]string{
				2: "tail-uid",
			},
			want: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mariadb := &mariadbv1alpha1.MariaDB{}
			mariadb.Annotations = tc.annotations

			got := isReplicaBootstrapScaleOutRecovery(mariadb, tc.fromIndex, tc.pvcUIDs)
			if got != tc.want {
				t.Fatalf("unexpected recovery scale-out detection: got %t, want %t", got, tc.want)
			}
		})
	}
}
