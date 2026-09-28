package replication

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	ds "github.com/mariadb-operator/mariadb-operator/v26/pkg/datastructures"
	mxsclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/maxscale/client"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testPrimaryNamespace = "default"
	testPrimaryMariaDB   = "mariadb-repl"
	testPrimaryMaxScale  = "maxscale-repl"
)

func testPrimaryMariaDBWithStatus(statusPrimaryPodIndex *int, maxScale bool) *mariadbv1alpha1.MariaDB {
	mdb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPrimaryMariaDB,
			Namespace: testPrimaryNamespace,
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 3,
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: statusPrimaryPodIndex,
		},
	}
	if maxScale {
		mdb.Spec.MaxScaleRef = &mariadbv1alpha1.ObjectReference{
			Name: testPrimaryMaxScale,
		}
	}
	return mdb
}

func testPrimaryMaxScaleWithServers() *mariadbv1alpha1.MaxScale {
	return &mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPrimaryMaxScale,
			Namespace: testPrimaryNamespace,
		},
		Spec: mariadbv1alpha1.MaxScaleSpec{
			Servers: []mariadbv1alpha1.MaxScaleServer{
				{Name: "mariadb-repl-0"},
				{Name: "mariadb-repl-1"},
				{Name: "mariadb-repl-2"},
			},
		},
	}
}

func testPrimaryServerAddress(mdb *mariadbv1alpha1.MariaDB, podIndex int) string {
	return statefulset.PodFQDNWithService(mdb.ObjectMeta, podIndex, mdb.InternalServiceKey().Name)
}

func testPrimaryServer(id, state, address string) mxsclient.Data[*mxsclient.ServerAttributes] {
	return mxsclient.Data[*mxsclient.ServerAttributes]{
		ID: id,
		Attributes: &mxsclient.ServerAttributes{
			State: state,
			Parameters: mxsclient.ServerParameters{
				Address: address,
			},
		},
	}
}

func TestPrimaryServerPodIndex(t *testing.T) {
	mdb := testPrimaryMariaDBWithStatus(ptr.To(0), true)
	mxs := testPrimaryMaxScaleWithServers()

	tests := []struct {
		name      string
		mariadb   *mariadbv1alpha1.MariaDB
		servers   []mxsclient.Data[*mxsclient.ServerAttributes]
		wantIndex *int
	}{
		{
			name:    "master matches status",
			mariadb: mdb,
			servers: []mxsclient.Data[*mxsclient.ServerAttributes]{
				testPrimaryServer("mariadb-repl-0", "Master, Running", testPrimaryServerAddress(mdb, 0)),
				testPrimaryServer("mariadb-repl-1", "Slave, Running", testPrimaryServerAddress(mdb, 1)),
				testPrimaryServer("mariadb-repl-2", "Slave, Running", testPrimaryServerAddress(mdb, 2)),
			},
			wantIndex: ptr.To(0),
		},
		{
			name:    "status is stale",
			mariadb: mdb,
			servers: []mxsclient.Data[*mxsclient.ServerAttributes]{
				testPrimaryServer("mariadb-repl-0", "Down", testPrimaryServerAddress(mdb, 0)),
				testPrimaryServer("mariadb-repl-1", "Master, Running", testPrimaryServerAddress(mdb, 1)),
				testPrimaryServer("mariadb-repl-2", "Slave, Running", testPrimaryServerAddress(mdb, 2)),
			},
			wantIndex: ptr.To(1),
		},
		{
			name:    "status not set",
			mariadb: testPrimaryMariaDBWithStatus(nil, true),
			servers: []mxsclient.Data[*mxsclient.ServerAttributes]{
				testPrimaryServer("mariadb-repl-2", "Master, Running", testPrimaryServerAddress(mdb, 2)),
			},
			wantIndex: ptr.To(2),
		},
		{
			name:    "no master",
			mariadb: mdb,
			servers: []mxsclient.Data[*mxsclient.ServerAttributes]{
				testPrimaryServer("mariadb-repl-0", "Down", testPrimaryServerAddress(mdb, 0)),
				testPrimaryServer("mariadb-repl-1", "Slave, Running", testPrimaryServerAddress(mdb, 1)),
				testPrimaryServer("mariadb-repl-2", "Running", testPrimaryServerAddress(mdb, 2)),
			},
			wantIndex: ptr.To(0),
		},
		{
			name:    "relay master is not master",
			mariadb: mdb,
			servers: []mxsclient.Data[*mxsclient.ServerAttributes]{
				testPrimaryServer("mariadb-repl-1", "Relay Master, Slave, Running", testPrimaryServerAddress(mdb, 1)),
			},
			wantIndex: ptr.To(0),
		},
		{
			name:    "master address not mapped to a Pod",
			mariadb: mdb,
			servers: []mxsclient.Data[*mxsclient.ServerAttributes]{
				testPrimaryServer("mariadb-repl-1", "Master, Running", "external.example.com"),
			},
			wantIndex: ptr.To(0),
		},
		{
			name:    "master not managed by MaxScale CR",
			mariadb: mdb,
			servers: []mxsclient.Data[*mxsclient.ServerAttributes]{
				testPrimaryServer("other-server", "Master, Running", testPrimaryServerAddress(mdb, 1)),
			},
			wantIndex: ptr.To(0),
		},
		{
			name:    "server without attributes",
			mariadb: mdb,
			servers: []mxsclient.Data[*mxsclient.ServerAttributes]{
				{ID: "mariadb-repl-1"},
			},
			wantIndex: ptr.To(0),
		},
		{
			name:      "no servers",
			mariadb:   mdb,
			wantIndex: ptr.To(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverIdx := ds.NewIndex(tt.servers, func(d mxsclient.Data[*mxsclient.ServerAttributes]) string {
				return d.ID
			})
			index := primaryServerPodIndex(tt.mariadb, mxs, serverIdx, logr.Discard())
			assert.Equal(t, tt.wantIndex, index)
		})
	}
}

func TestCurrentPrimaryPodIndex(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding to scheme: %v", err)
	}

	// A monitor other than mariadbmon has no cooperative monitoring lock holder, so no MaxScale API request is made.
	mxsNoPrimaryClient := testPrimaryMaxScaleWithServers()
	mxsNoPrimaryClient.Spec.Monitor.Module = mariadbv1alpha1.MonitorModuleGalera

	tests := []struct {
		name      string
		mariadb   *mariadbv1alpha1.MariaDB
		objs      []ctrlclient.Object
		wantIndex *int
	}{
		{
			name:      "MaxScale not enabled",
			mariadb:   testPrimaryMariaDBWithStatus(ptr.To(1), false),
			wantIndex: ptr.To(1),
		},
		{
			name:      "MaxScale not enabled and status not set",
			mariadb:   testPrimaryMariaDBWithStatus(nil, false),
			wantIndex: nil,
		},
		{
			name:      "MaxScale not found",
			mariadb:   testPrimaryMariaDBWithStatus(ptr.To(1), true),
			wantIndex: ptr.To(1),
		},
		{
			name:      "MaxScale primary client not found",
			mariadb:   testPrimaryMariaDBWithStatus(ptr.To(2), true),
			objs:      []ctrlclient.Object{mxsNoPrimaryClient},
			wantIndex: ptr.To(2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objs...).Build()
			r := &ReplicationReconciler{
				refResolver: refresolver.New(client),
			}
			index, err := r.currentPrimaryPodIndex(context.Background(), tt.mariadb, logr.Discard())
			assert.NoError(t, err)
			assert.Equal(t, tt.wantIndex, index)
		})
	}
}
