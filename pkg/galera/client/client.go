package client

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/galera/recovery"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
)

func IsPodHealthy(ctx context.Context, sqlClient *sql.Client) (bool, error) {
	status, err := sqlClient.GaleraClusterStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting cluster status: %v", err)
	}

	return status == "Primary", nil
}

var (
	GaleraStateSynced string = "Synced"
	GaleraStateDonor  string = "Donor/Desynced"
)

func IsPodSynced(ctx context.Context, sqlClient *sql.Client) (bool, error) {
	healthy, err := IsPodHealthy(ctx, sqlClient)
	if err != nil {
		return false, fmt.Errorf("error checking Pod health: %v", err)
	}
	if !healthy {
		return false, nil
	}

	state, err := sqlClient.GaleraLocalState(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting local state: %v", err)
	}

	return state == GaleraStateSynced, nil
}

// IsPodSyncedWithUUID reports whether the Pod is Synced in the Primary component and its
// wsrep_cluster_state_uuid matches expectedUUID.
func IsPodSyncedWithUUID(ctx context.Context, sqlClient *sql.Client, expectedUUID string) (bool, error) {
	synced, err := IsPodSynced(ctx, sqlClient)
	if err != nil {
		return false, err
	}
	if !synced {
		return false, nil
	}
	if expectedUUID == "" {
		return false, fmt.Errorf("expected cluster state UUID must be set")
	}

	uuid, err := sqlClient.GaleraClusterStateUUID(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting cluster state UUID: %v", err)
	}
	if err := ValidateClusterStateUUID(uuid); err != nil {
		return false, err
	}
	return uuid == expectedUUID, nil
}

// ValidateClusterStateUUID returns an error when uuid is empty or the Galera zero UUID.
func ValidateClusterStateUUID(uuid string) error {
	if uuid == "" {
		return fmt.Errorf("cluster state UUID must not be empty")
	}
	if uuid == recovery.ZeroUUID {
		return fmt.Errorf("cluster state UUID must not be %s", recovery.ZeroUUID)
	}
	return nil
}

// CoherentClusterStateUUID returns the shared UUID when all values agree and are valid,
// or an error when the set is empty, contains an invalid UUID, or has conflicting UUIDs.
func CoherentClusterStateUUID(uuids []string) (string, error) {
	if len(uuids) == 0 {
		return "", fmt.Errorf("no cluster state UUIDs provided")
	}
	var expected string
	for i, uuid := range uuids {
		if err := ValidateClusterStateUUID(uuid); err != nil {
			return "", fmt.Errorf("invalid cluster state UUID at index %d: %w", i, err)
		}
		if i == 0 {
			expected = uuid
			continue
		}
		if uuid != expected {
			return "", fmt.Errorf("cluster state UUID mismatch: %s != %s", expected, uuid)
		}
	}
	return expected, nil
}
