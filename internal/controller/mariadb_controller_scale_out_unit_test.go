package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestIsScalingOutContinuesDuringReplicaRecoveryScaleOut(t *testing.T) {
	reconciler := &MariaDBReconciler{}
	mariadb := &mariadbv1alpha1.MariaDB{
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 3,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:   mariadbv1alpha1.ConditionTypeScaledOut,
					Status: metav1.ConditionFalse,
				},
				{
					Type:   mariadbv1alpha1.ConditionTypeReplicaRecovered,
					Status: metav1.ConditionFalse,
				},
			},
		},
	}
	sts := &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			Replicas:      2,
			ReadyReplicas: 2,
		},
	}

	isScalingOut, err := reconciler.isScalingOut(mariadb, sts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isScalingOut {
		t.Fatalf("expected ongoing scale out to continue while replica recovery condition is false")
	}
}
