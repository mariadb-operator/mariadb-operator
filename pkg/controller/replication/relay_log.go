package replication

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/replication"
)

// HasRelayLogEvents indicates that there are events in the IO thread to be applied by the SQL thread.
func HasRelayLogEvents(status *mariadbv1alpha1.ReplicaStatusVars, gtidDomainId uint32,
	logger logr.Logger) (bool, error) {
	if status.GtidIOPos == nil {
		return false, errors.New("GTID IO position must be set")
	}
	if status.GtidCurrentPos == nil {
		return false, errors.New("GTID SQL position must be set")
	}

	gtidIOPos, err := replication.ParseGtid(*status.GtidIOPos, gtidDomainId, logger)
	if err != nil {
		return false, fmt.Errorf("error parsing GTID IO position: %v", err)
	}
	gtidCurrentPos, err := replication.ParseGtid(*status.GtidCurrentPos, gtidDomainId, logger)
	if err != nil {
		return false, fmt.Errorf("error parsing GTID SQL position: %v", err)
	}

	if gtidIOPos.Equal(gtidCurrentPos) {
		return false, nil
	}
	greaterThan, err := gtidIOPos.GreaterThan(gtidCurrentPos)
	if err != nil {
		return false, fmt.Errorf("error comparing GTID IO and SQL positions: %v", err)
	}
	if greaterThan {
		logger.Info(
			"Detected events in relay log. Skipping...",
			"gtid-io-pos", gtidIOPos.String(),
			"gtid-current-pos", gtidCurrentPos.String(),
		)
		return true, nil
	}

	logger.Info(
		"GTID SQL position ahead of IO (unexpected state)",
		"gtid-io-pos", gtidIOPos.String(),
		"gtid-current-pos", gtidCurrentPos.String(),
	)
	return false, nil
}
