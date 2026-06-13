package replication

import (
	"errors"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
)

// HasRelayLogEvents indicates that the IO and SQL GTID sets are not exactly synced.
func HasRelayLogEvents(status *mariadbv1alpha1.ReplicaStatusVars, gtidDomainId uint32,
	logger logr.Logger) (bool, error) {
	if status.GtidIOPos == nil {
		return false, errors.New("GTID IO position must be set")
	}
	if status.GtidCurrentPos == nil {
		return false, errors.New("GTID SQL position must be set")
	}

	gtidIOPos, err := sortedDomainGtidSet(*status.GtidIOPos, gtidDomainId, logger)
	if err != nil {
		return false, fmt.Errorf("error parsing GTID IO position: %v", err)
	}
	gtidCurrentPos, err := sortedDomainGtidSet(*status.GtidCurrentPos, gtidDomainId, logger)
	if err != nil {
		return false, fmt.Errorf("error parsing GTID SQL position: %v", err)
	}

	if equalGtidSets(gtidIOPos, gtidCurrentPos) {
		return false, nil
	}

	logger.Info(
		"Detected unsynced GTID positions. Skipping...",
		"gtid-io-pos", gtidIOPos,
		"gtid-current-pos", gtidCurrentPos,
	)
	return true, nil
}

func sortedDomainGtidSet(rawGtid string, domainId uint32, logger logr.Logger) ([]string, error) {
	gtids, err := replication.ParseGtidsWithDomainId(rawGtid, domainId, logger)
	if err != nil {
		return nil, err
	}
	set := make([]string, 0, len(gtids))
	for _, gtid := range gtids {
		set = append(set, gtid.String())
	}
	sort.Strings(set)
	return set, nil
}

func equalGtidSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
