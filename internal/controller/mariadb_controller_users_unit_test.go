package controller

import (
	"context"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWaitForGrant(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}

	key := types.NamespacedName{
		Name:      "test-grant",
		Namespace: "test",
	}

	testCases := map[string]struct {
		grant       *mariadbv1alpha1.Grant
		wantRequeue bool
	}{
		"missing grant requeues": {
			wantRequeue: true,
		},
		"not ready grant requeues": {
			grant: &mariadbv1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
			},
			wantRequeue: true,
		},
		"ready grant continues": {
			grant: &mariadbv1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Status: mariadbv1alpha1.GrantStatus{
					Conditions: []metav1.Condition{
						{
							Type:   mariadbv1alpha1.ConditionTypeReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tc.grant != nil {
				clientBuilder = clientBuilder.WithObjects(tc.grant)
			}

			reconciler := &MariaDBReconciler{
				Client: clientBuilder.Build(),
			}

			result, err := reconciler.waitForGrant(context.Background(), key, "test Grant")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantRequeue {
				if result.RequeueAfter != time.Second {
					t.Fatalf("expected requeue after %v, got %v", time.Second, result.RequeueAfter)
				}
				return
			}

			if !result.IsZero() {
				t.Fatalf("expected zero result, got %+v", result)
			}
		})
	}
}
