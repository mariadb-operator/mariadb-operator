package galera

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestShouldReconcileRecovery(t *testing.T) {
	tests := []struct {
		name   string
		mdb    *mariadbv1alpha1.MariaDB
		sts    *appsv1.StatefulSet
		wantOk bool
	}{
		{
			name:   "no Galera conditions",
			mdb:    testMariadbWithGaleraCondition(nil),
			sts:    testStatefulSetWithReadyReplicas(3),
			wantOk: false,
		},
		{
			name:   "Galera ready",
			mdb:    testMariadbWithGaleraCondition(ptr.To(metav1.ConditionTrue)),
			sts:    testStatefulSetWithReadyReplicas(3),
			wantOk: false,
		},
		{
			name:   "Galera not ready and unhealthy cluster",
			mdb:    testMariadbWithGaleraCondition(ptr.To(metav1.ConditionFalse)),
			sts:    testStatefulSetWithReadyReplicas(0),
			wantOk: true,
		},
		{
			name:   "Galera not ready and partially healthy cluster",
			mdb:    testMariadbWithGaleraCondition(ptr.To(metav1.ConditionFalse)),
			sts:    testStatefulSetWithReadyReplicas(2),
			wantOk: true,
		},
		{
			name:   "Galera not ready and healthy cluster",
			mdb:    testMariadbWithGaleraCondition(ptr.To(metav1.ConditionFalse)),
			sts:    testStatefulSetWithReadyReplicas(3),
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOk := shouldReconcileRecovery(tt.mdb, tt.sts)
			if tt.wantOk != gotOk {
				t.Errorf("unexpected shouldReconcileRecovery value: expected: %v, got: %v", tt.wantOk, gotOk)
			}
		})
	}
}

func TestIsClusterHealthy(t *testing.T) {
	tests := []struct {
		name   string
		mdb    *mariadbv1alpha1.MariaDB
		sts    *appsv1.StatefulSet
		wantOk bool
	}{
		{
			name:   "no ready replicas",
			mdb:    testMariadbWithGaleraCondition(nil),
			sts:    testStatefulSetWithReadyReplicas(0),
			wantOk: false,
		},
		{
			name:   "some ready replicas",
			mdb:    testMariadbWithGaleraCondition(nil),
			sts:    testStatefulSetWithReadyReplicas(2),
			wantOk: false,
		},
		{
			name:   "all ready replicas",
			mdb:    testMariadbWithGaleraCondition(nil),
			sts:    testStatefulSetWithReadyReplicas(3),
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOk := isClusterHealthy(tt.mdb, tt.sts)
			if tt.wantOk != gotOk {
				t.Errorf("unexpected isClusterHealthy value: expected: %v, got: %v", tt.wantOk, gotOk)
			}
		})
	}
}

func testMariadbWithGaleraCondition(galeraReady *metav1.ConditionStatus) *mariadbv1alpha1.MariaDB {
	mdb := &mariadbv1alpha1.MariaDB{
		Spec: mariadbv1alpha1.MariaDBSpec{
			Galera: &mariadbv1alpha1.Galera{
				Enabled: true,
				GaleraSpec: mariadbv1alpha1.GaleraSpec{
					Recovery: &mariadbv1alpha1.GaleraRecovery{
						Enabled: true,
					},
				},
			},
			Replicas: 3,
		},
	}
	if galeraReady != nil {
		mdb.Status.Conditions = []metav1.Condition{
			{
				Type:   mariadbv1alpha1.ConditionTypeGaleraReady,
				Status: *galeraReady,
				Reason: "test",
			},
		}
	}
	return mdb
}

func testStatefulSetWithReadyReplicas(readyReplicas int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: readyReplicas,
		},
	}
}
