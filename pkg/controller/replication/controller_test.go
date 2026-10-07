package replication

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	conditions "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestShouldReconcileReplication(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, clientgoscheme.AddToScheme(scheme))
	assert.NoError(t, mariadbv1alpha1.AddToScheme(scheme))

	mariadbMeta := metav1.ObjectMeta{
		Name:      "mariadb-repl",
		Namespace: "default",
	}
	mariadb := func(maxScale bool, replicationConfigured bool) *mariadbv1alpha1.MariaDB {
		mdb := &mariadbv1alpha1.MariaDB{
			ObjectMeta: mariadbMeta,
			Spec: mariadbv1alpha1.MariaDBSpec{
				Replicas: 3,
				Replication: &mariadbv1alpha1.Replication{
					Enabled: true,
				},
			},
			Status: mariadbv1alpha1.MariaDBStatus{
				CurrentPrimaryPodIndex: ptr.To(0),
				CurrentPrimary:         ptr.To(statefulset.PodName(mariadbMeta, 0)),
			},
		}
		if maxScale {
			mdb.Spec.MaxScaleRef = &mariadbv1alpha1.ObjectReference{
				Name: "maxscale-repl",
			}
		}
		if replicationConfigured {
			conditions.SetReplicationConfigured(&mdb.Status)
		}
		return mdb
	}
	maxscale := func(states ...string) *mariadbv1alpha1.MaxScale {
		mdb := mariadb(true, true)
		mxs := &mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "maxscale-repl",
				Namespace: mariadbMeta.Namespace,
			},
		}
		for i, state := range states {
			name := statefulset.PodName(mariadbMeta, i)
			mxs.Spec.Servers = append(mxs.Spec.Servers, mariadbv1alpha1.MaxScaleServer{
				Name:    name,
				Address: statefulset.PodFQDNWithService(mariadbMeta, i, mdb.InternalServiceKey().Name),
				Port:    3306,
			})
			mxs.Status.Servers = append(mxs.Status.Servers, mariadbv1alpha1.MaxScaleServerStatus{
				Name:  name,
				State: state,
			})
		}
		return mxs
	}
	primaryPod := func(ready bool) *corev1.Pod {
		status := corev1.ConditionFalse
		if ready {
			status = corev1.ConditionTrue
		}
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      statefulset.PodName(mariadbMeta, 0),
				Namespace: mariadbMeta.Namespace,
			},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: status,
					},
				},
			},
		}
	}
	switchingMaxScale := maxscale("Master, Running", "Slave, Running", "Slave, Running")
	conditions.SetPrimarySwitching(&switchingMaxScale.Status, statefulset.PodName(mariadbMeta, 1))

	tests := []struct {
		name       string
		mariadb    *mariadbv1alpha1.MariaDB
		objects    []client.Object
		wantResult ctrl.Result
	}{
		{
			name: "no current primary",
			mariadb: func() *mariadbv1alpha1.MariaDB {
				mdb := mariadb(false, false)
				mdb.Status.CurrentPrimaryPodIndex = nil
				return mdb
			}(),
			wantResult: ctrl.Result{RequeueAfter: 1 * time.Second},
		},
		{
			name:       "no MaxScale and primary not ready",
			mariadb:    mariadb(false, true),
			objects:    []client.Object{primaryPod(false)},
			wantResult: ctrl.Result{},
		},
		{
			name:       "MaxScale not found",
			mariadb:    mariadb(true, true),
			objects:    []client.Object{primaryPod(true)},
			wantResult: ctrl.Result{},
		},
		{
			name:    "MaxScale and primary ready",
			mariadb: mariadb(true, true),
			objects: []client.Object{
				primaryPod(true),
				maxscale("Master, Running", "Slave, Running", "Slave, Running"),
			},
			wantResult: ctrl.Result{},
		},
		{
			name:    "MaxScale switching primary",
			mariadb: mariadb(true, true),
			objects: []client.Object{
				primaryPod(true),
				switchingMaxScale,
			},
			wantResult: ctrl.Result{RequeueAfter: 5 * time.Second},
		},
		{
			name:    "MaxScale and primary Pod not found",
			mariadb: mariadb(true, true),
			objects: []client.Object{
				maxscale("Master, Running", "Slave, Running", "Slave, Running"),
			},
			wantResult: ctrl.Result{RequeueAfter: 5 * time.Second},
		},
		{
			name:    "MaxScale and primary Pod not ready",
			mariadb: mariadb(true, true),
			objects: []client.Object{
				primaryPod(false),
				maxscale("Down", "Slave, Running", "Slave, Running"),
			},
			wantResult: ctrl.Result{RequeueAfter: 5 * time.Second},
		},
		{
			name:    "MaxScale reports primary down and primary Pod ready",
			mariadb: mariadb(true, true),
			objects: []client.Object{
				primaryPod(true),
				maxscale("Down", "Running", "Slave, Running"),
			},
			wantResult: ctrl.Result{RequeueAfter: 5 * time.Second},
		},
		{
			// The Maintenance phase that disables read_only runs after this one, so it must not be blocked.
			name:    "MaxScale reports read_only primary as running",
			mariadb: mariadb(true, true),
			objects: []client.Object{
				primaryPod(true),
				maxscale("Running", "Slave, Running", "Slave, Running"),
			},
			wantResult: ctrl.Result{},
		},
		{
			name:    "MaxScale and primary not ready before replication is configured",
			mariadb: mariadb(true, false),
			objects: []client.Object{
				maxscale("Down", "Down", "Down"),
			},
			wantResult: ctrl.Result{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()
			r := &ReplicationReconciler{
				Client:      client,
				refResolver: refresolver.New(client),
			}
			req := &ReconcileRequest{
				mariadb: tt.mariadb,
			}

			result, err := r.shouldReconcileReplication(context.Background(), req, logr.Discard())
			assert.NoError(t, err)
			assert.Equal(t, tt.wantResult, result, "Result mismatch")
		})
	}
}
