package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	mxsclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/maxscale/client"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHasStaleMonitorTopology(t *testing.T) {
	newMariaDB := func(roles map[string]mariadbv1alpha1.ReplicationRole) *mariadbv1alpha1.MariaDB {
		return &mariadbv1alpha1.MariaDB{
			Spec: mariadbv1alpha1.MariaDBSpec{
				Replication: &mariadbv1alpha1.Replication{
					Enabled: true,
				},
			},
			Status: mariadbv1alpha1.MariaDBStatus{
				CurrentPrimary: ptr.To("db-1"),
				Replication: &mariadbv1alpha1.ReplicationStatus{
					Roles: roles,
				},
			},
		}
	}
	newMaxScale := func(servers ...mariadbv1alpha1.MaxScaleServerStatus) *mariadbv1alpha1.MaxScale {
		return &mariadbv1alpha1.MaxScale{
			Status: mariadbv1alpha1.MaxScaleStatus{
				Servers: servers,
			},
		}
	}
	healthyRoles := map[string]mariadbv1alpha1.ReplicationRole{
		"db-0": mariadbv1alpha1.ReplicationRolePrimary,
		"db-1": mariadbv1alpha1.ReplicationRoleReplica,
	}

	testCases := map[string]struct {
		maxscale *mariadbv1alpha1.MaxScale
		mariadb  *mariadbv1alpha1.MariaDB
		want     bool
	}{
		"observed primary is not Master in the pool": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Maintenance, Running"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Slave, Running"},
			),
			mariadb: newMariaDB(healthyRoles),
			want:    true,
		},
		"pool holds a Master": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Master, Running"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Slave, Running"},
			),
			mariadb: newMariaDB(healthyRoles),
			want:    false,
		},
		"observed primary is down in the pool": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Down"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Slave, Running"},
			),
			mariadb: newMariaDB(healthyRoles),
			want:    false,
		},
		"no observed primary": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Running"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Running"},
			),
			mariadb: newMariaDB(map[string]mariadbv1alpha1.ReplicationRole{
				"db-0": mariadbv1alpha1.ReplicationRoleUnknown,
				"db-1": mariadbv1alpha1.ReplicationRoleUnknown,
			}),
			want: false,
		},
		"switchover in progress is left alone": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Maintenance, Running"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Slave, Running"},
			),
			mariadb: func() *mariadbv1alpha1.MariaDB {
				mdb := newMariaDB(healthyRoles)
				condition.SetPrimarySwitching(&mdb.Status, "db-0")
				return mdb
			}(),
			want: false,
		},
		"replication disabled": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Running"},
			),
			mariadb: &mariadbv1alpha1.MariaDB{},
			want:    false,
		},
		"nil MariaDB": {
			maxscale: newMaxScale(),
			mariadb:  nil,
			want:     false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := hasStaleMonitorTopology(tc.maxscale, tc.mariadb); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestEnsurePrimaryServerLiftsMaintenanceFromObservedPrimary(t *testing.T) {
	const namespace = "default"
	serverStates := map[string]string{
		"db-0": "Maintenance, Running",
		"db-1": "Slave, Running",
	}
	var (
		mu       sync.Mutex
		requests []string
	)
	mxsAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		name := strings.TrimPrefix(r.URL.Path, "/servers/")
		switch r.Method {
		case http.MethodGet:
			state, ok := serverStates[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(mxsclient.Object[*mxsclient.ServerAttributes]{
				Data: mxsclient.Data[*mxsclient.ServerAttributes]{
					ID:         name,
					Type:       mxsclient.ObjectTypeServers,
					Attributes: &mxsclient.ServerAttributes{State: state},
				},
			})
		case http.MethodPut:
			requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer mxsAPI.Close()

	podClient, err := mxsclient.NewClient(mxsAPI.URL)
	if err != nil {
		t.Fatalf("error creating MaxScale client: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db",
			Namespace: namespace,
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimary: ptr.To("db-0"),
			Replication: &mariadbv1alpha1.ReplicationStatus{
				Roles: map[string]mariadbv1alpha1.ReplicationRole{
					"db-0": mariadbv1alpha1.ReplicationRolePrimary,
					"db-1": mariadbv1alpha1.ReplicationRoleReplica,
				},
			},
		},
	}
	maxscale := &mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "maxscale",
			Namespace: namespace,
		},
		Spec: mariadbv1alpha1.MaxScaleSpec{
			MariaDBRef: &mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name: mariadb.Name,
				},
			},
			Replicas: 1,
			Servers: []mariadbv1alpha1.MaxScaleServer{
				{Name: "db-0"},
				{Name: "db-1"},
			},
		},
		Status: mariadbv1alpha1.MaxScaleStatus{
			Servers: []mariadbv1alpha1.MaxScaleServerStatus{
				{Name: "db-0", State: serverStates["db-0"]},
				{Name: "db-1", State: serverStates["db-1"]},
			},
		},
	}

	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, maxscale).
		WithStatusSubresource(mariadb, maxscale).
		Build()
	reconciler := &MaxScaleReconciler{
		Client:      fakeClient,
		RefResolver: refresolver.New(fakeClient),
		Recorder:    events.NewFakeRecorder(10),
	}
	req := &requestMaxScale{
		mxs: maxscale,
		podClientSet: map[string]*mxsclient.Client{
			"maxscale-0": podClient,
		},
	}

	result, err := reconciler.ensurePrimaryServer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsZero() {
		t.Error("expected a requeue while the pool has no primary server")
	}

	mu.Lock()
	defer mu.Unlock()
	wantClear := "/servers/db-0/clear?state=maintenance"
	cleared := false
	for _, r := range requests {
		if r == wantClear {
			cleared = true
		}
		if strings.HasPrefix(r, "/servers/db-0/set") {
			t.Errorf("expected the observed primary to stay out of maintenance, got %q", r)
		}
	}
	if !cleared {
		t.Fatalf("expected %q while the pool had no primary server, got %v", wantClear, requests)
	}
}
