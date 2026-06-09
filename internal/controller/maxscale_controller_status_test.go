package controller

import (
	"context"
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	ds "github.com/mariadb-operator/mariadb-operator/v26/pkg/datastructures"
	mxsclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/maxscale/client"
)

func TestServerStatusFromIndexSortsByName(t *testing.T) {
	idx := ds.Index[mxsclient.Data[*mxsclient.ServerAttributes]]{
		"zeta": {
			ID:         "zeta",
			Attributes: &mxsclient.ServerAttributes{State: "Slave, Running"},
		},
		"beta": {
			ID:         "beta",
			Attributes: &mxsclient.ServerAttributes{State: "Master, Running"},
		},
		"alpha": {
			ID:         "alpha",
			Attributes: &mxsclient.ServerAttributes{State: "Master, Running"},
		},
	}

	got := serverStatusFromIndex(idx)
	wantServers := []mariadbv1alpha1.MaxScaleServerStatus{
		{Name: "alpha", State: "Master, Running"},
		{Name: "beta", State: "Master, Running"},
		{Name: "zeta", State: "Slave, Running"},
	}

	if got.primary != "alpha" {
		t.Fatalf("expected primary alpha, got %q", got.primary)
	}
	if !reflect.DeepEqual(got.servers, wantServers) {
		t.Fatalf("expected servers %#v, got %#v", wantServers, got.servers)
	}
}

func TestResourceStatusFromIndexSortsByName(t *testing.T) {
	idx := ds.Index[mxsclient.Data[*mxsclient.ServiceAttributes]]{
		"read-write": {
			ID:         "read-write",
			Attributes: &mxsclient.ServiceAttributes{State: "Started"},
		},
		"read-only": {
			ID:         "read-only",
			Attributes: &mxsclient.ServiceAttributes{State: "Started"},
		},
	}

	got := resourceStatusFromIndex(idx, func(attrs *mxsclient.ServiceAttributes) string {
		return attrs.State
	})
	want := []mariadbv1alpha1.MaxScaleResourceStatus{
		{Name: "read-only", State: "Started"},
		{Name: "read-write", State: "Started"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected resource statuses %#v, got %#v", want, got)
	}
}

func TestPatchStatusSkipsUnchangedStatus(t *testing.T) {
	mxs := &mariadbv1alpha1.MaxScale{
		Status: mariadbv1alpha1.MaxScaleStatus{
			Replicas: 3,
		},
	}

	err := (&MaxScaleReconciler{}).patchStatus(context.Background(), mxs, func(status *mariadbv1alpha1.MaxScaleStatus) error {
		status.Replicas = 3
		return nil
	})
	if err != nil {
		t.Fatalf("expected unchanged status patch to be skipped, got error: %v", err)
	}
}
