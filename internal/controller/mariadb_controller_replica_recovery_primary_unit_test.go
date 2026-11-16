package controller

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDataHoldingCurrentPrimary(t *testing.T) {
	tests := []struct {
		name      string
		storedUID string
		pvcUIDs   map[int]string
		want      string
	}{
		{
			name:      "primary with its own storage",
			storedUID: "primary-uid",
			pvcUIDs:   map[int]string{0: "replica-uid", 1: "primary-uid"},
			want:      "db-cluster-1",
		},
		{
			name:    "primary storage not tracked yet",
			pvcUIDs: map[int]string{1: "primary-uid"},
			want:    "db-cluster-1",
		},
		{
			name:      "primary storage replaced",
			storedUID: "primary-uid",
			pvcUIDs:   map[int]string{1: "fresh-uid"},
		},
		{
			name:      "primary storage missing",
			storedUID: "primary-uid",
			pvcUIDs:   map[int]string{0: "replica-uid"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mariadb := preserveTestMariaDB(nil)
			if tt.storedUID != "" {
				mariadb.Annotations[storagePVCUIDAnnotationKey(1)] = tt.storedUID
			}
			if got := dataHoldingCurrentPrimary(mariadb, tt.pvcUIDs); got != tt.want {
				t.Fatalf("unexpected primary: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExcludeCurrentPrimaryFromRecovery(t *testing.T) {
	tests := []struct {
		name             string
		replicaToRecover string
		wantActive       []string
		wantCleared      bool
		wantEvent        bool
	}{
		{
			name:             "recovery of a replica promoted since is ended",
			replicaToRecover: "db-cluster-1",
			wantActive:       []string{},
			wantCleared:      true,
			wantEvent:        true,
		},
		{
			name:             "recovery of a replica goes on",
			replicaToRecover: "db-cluster-0",
			wantActive:       []string{"db-cluster-0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mariadb := preserveTestMariaDB(nil)
			mariadb.Annotations[storagePVCUIDAnnotationKey(1)] = "primary-uid"
			mariadb.SetReplicaToRecover(ptr.To(tt.replicaToRecover))
			fakeClient := fake.NewClientBuilder().WithScheme(preserveTestScheme(t)).
				WithObjects(mariadb).WithStatusSubresource(mariadb).Build()
			recorder := events.NewFakeRecorder(10)
			r := &MariaDBReconciler{Client: fakeClient, Recorder: recorder}

			active, primary, err := r.excludeCurrentPrimaryFromRecovery(context.Background(), mariadb,
				map[int]string{0: "replica-uid", 1: "primary-uid"}, getActiveReplicaRecoveryReplicas(mariadb), logr.Discard())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if primary != "db-cluster-1" {
				t.Fatalf("unexpected current primary: %q", primary)
			}
			if strings.Join(active, ",") != strings.Join(tt.wantActive, ",") {
				t.Fatalf("unexpected active recovery replicas: %v, want %v", active, tt.wantActive)
			}
			var stored mariadbv1alpha1.MariaDB
			if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &stored); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cleared := len(getActiveReplicaRecoveryReplicas(&stored)) == 0; cleared != tt.wantCleared {
				t.Fatalf("unexpected replica to recover: %v", stored.Status.Replication)
			}
			select {
			case event := <-recorder.Events:
				if !tt.wantEvent || !strings.Contains(event, "it became the current primary") {
					t.Fatalf("unexpected event: %s", event)
				}
			default:
				if tt.wantEvent {
					t.Fatal("expected an event")
				}
			}
		})
	}
}

func TestWithoutReplica(t *testing.T) {
	replicas := []string{"db-cluster-0", "db-cluster-1"}
	if got := withoutReplica(replicas, "db-cluster-1"); strings.Join(got, ",") != "db-cluster-0" {
		t.Fatalf("unexpected replicas: %v", got)
	}
	if got := withoutReplica(replicas, ""); strings.Join(got, ",") != "db-cluster-0,db-cluster-1" {
		t.Fatalf("unexpected replicas: %v", got)
	}
	if strings.Join(replicas, ",") != "db-cluster-0,db-cluster-1" {
		t.Fatalf("input modified: %v", replicas)
	}
}
