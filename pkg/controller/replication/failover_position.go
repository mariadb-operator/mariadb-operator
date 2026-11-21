package replication

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
)

// failoverReplicaOpts returns the options to configure a replica of a primary that failed, so it resumes from the first
// GTID of the promoted primary. Without log_slave_updates, the promoted primary's binary log starts at its promotion, and a
// replica requesting its own position is either rejected with error 1236 or, when the promoted primary kept binary logs from
// an earlier primary term, replays them from the start.
//
// This is only safe when the replica applied exactly what the promoted primary had applied. Otherwise no options are
// returned, and a replica that is behind or diverged fails visibly instead of skipping or replaying transactions.
func failoverReplicaOpts(ctx context.Context, client *sql.Client, position *mariadbv1alpha1.FailoverPositionStatus,
	logger logr.Logger) ([]ConfigureReplicaOpt, error) {
	domainId, err := client.GtidDomainId(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting gtid_domain_id: %v", err)
	}
	currentGtid, err := client.GtidCurrentPos(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting gtid_current_pos: %v", err)
	}
	if !gtidsEqualInDomain(currentGtid, position.AppliedGtid, *domainId, logger) {
		logger.Info("Replica position does not match the failover position", "gtid", currentGtid,
			"applied-gtid", position.AppliedGtid)
		return nil, nil
	}

	changeMasterGtid, err := mariadbv1alpha1.GtidSlavePos.MariaDBFormat()
	if err != nil {
		return nil, fmt.Errorf("error getting change master GTID: %v", err)
	}
	logger.Info("Resuming replica from failover position", "gtid", currentGtid, "promotion-gtid", position.PromotionGtid)
	return []ConfigureReplicaOpt{
		WithGtidSlavePos(position.PromotionGtid),
		WithChangeMasterOpts(
			sql.WithChangeMasterGtid(changeMasterGtid),
		),
	}, nil
}

func gtidsEqualInDomain(gtid, otherGtid string, domainId uint32, logger logr.Logger) bool {
	if gtid == "" || otherGtid == "" {
		return false
	}
	parsed, err := replication.ParseGtidWithDomainId(gtid, domainId, logger)
	if err != nil {
		return false
	}
	otherParsed, err := replication.ParseGtidWithDomainId(otherGtid, domainId, logger)
	if err != nil {
		return false
	}
	return parsed.Equal(otherParsed)
}
