package replication

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"text/template"

	env "github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
)

func NewReplicationConfig(env *env.PodEnvironment) ([]byte, error) {
	replEnabled, err := env.IsReplEnabled()
	if err != nil {
		return nil, fmt.Errorf("error checking if replication is enabled: %v", err)
	}
	if !replEnabled {
		return nil, errors.New("replication must be enabled")
	}
	gtidStrictMode, err := env.ReplGtidStrictMode()
	if err != nil {
		return nil, fmt.Errorf("error getting GTID strict mode: %v", err)
	}
	gtidDomainID, err := gtidDomainID(env.MariaDBReplGtidDomainID)
	if err != nil {
		return nil, fmt.Errorf("error getting GTID domain ID: %v", err)
	}
	semiSyncEnabled, err := env.ReplSemiSyncEnabled()
	if err != nil {
		return nil, fmt.Errorf("error getting semi-sync enabled: %v", err)
	}
	semiSyncMasterTimeout, err := env.ReplSemiSyncMasterTimeout()
	if err != nil {
		return nil, fmt.Errorf("error getting semi-sync master timeout: %v", err)
	}
	serverIDStartIndex, err := serverIDStartIndex(env.MariaDBReplServerIDStartIndex)
	if err != nil {
		return nil, fmt.Errorf("error getting server ID start index: %v", err)
	}
	serverID, err := serverId(env.PodName, serverIDStartIndex)
	if err != nil {
		return nil, fmt.Errorf("error getting server ID: %v", err)
	}
	syncBinlog, err := env.ReplSyncBinlog()
	if err != nil {
		return nil, fmt.Errorf("error getting master sync binlog: %v", err)
	}

	// To facilitate switchover/failover and avoid clashing with MaxScale, this configuration allows any Pod to act either as a primary or a replica.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication#enabling-semisynchronous-replication
	tpl := createTpl("replication", `[mariadb]
log_bin
log_basename={{.LogName }}
{{- with .GtidStrictMode }}
gtid_strict_mode
{{- end }}
{{- with .GtidDomainID }}
gtid_domain_id={{ . }}
{{- end }}
{{- if .SemiSyncEnabled }}
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
{{- with .SemiSyncMasterTimeout }}
rpl_semi_sync_master_timeout={{ . }}
{{- end }}
{{- with .SemiSyncMasterWaitPoint }}
rpl_semi_sync_master_wait_point={{ . }}
{{- end }}
{{- end }}
server_id={{ .ServerID }}
{{- with .SyncBinlog }}
sync_binlog={{ . }}
{{- end }}
`)
	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, struct {
		LogName                 string
		GtidStrictMode          bool
		GtidDomainID            *int
		SemiSyncEnabled         bool
		SemiSyncMasterTimeout   *int64
		SemiSyncMasterWaitPoint string
		SyncBinlog              *int
		ServerID                int
	}{
		LogName:                 env.MariadbName,
		GtidStrictMode:          gtidStrictMode,
		GtidDomainID:            gtidDomainID,
		SemiSyncEnabled:         semiSyncEnabled,
		SemiSyncMasterTimeout:   semiSyncMasterTimeout,
		SemiSyncMasterWaitPoint: env.MariaDBReplSemiSyncMasterWaitPoint,
		ServerID:                serverID,
		SyncBinlog:              syncBinlog,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gtidDomainID(rawGtidDomainID string) (*int, error) {
	if rawGtidDomainID == "" {
		return nil, nil // rely on server defaults
	}
	gtidDomainId, err := strconv.Atoi(rawGtidDomainID)
	if err != nil {
		return nil, fmt.Errorf("error parsing GTID domain ID: %v", err)
	}
	return &gtidDomainId, nil
}

func serverIDStartIndex(rawStartIndex string) (int, error) {
	if rawStartIndex == "" {
		return 10, nil
	}
	startIndex, err := strconv.Atoi(rawStartIndex)
	if err != nil {
		return 0, fmt.Errorf("serverID start index could not be parsed. %v", err)
	}
	return startIndex, nil
}

func serverId(podName string, startIndex int) (int, error) {
	podIndex, err := statefulset.PodIndex(podName)
	if err != nil {
		return 0, fmt.Errorf("error getting Pod index: %v", err)
	}
	return startIndex + *podIndex, nil
}

func formatAccountName(username, host string) string {
	return fmt.Sprintf("'%s'@'%s'", username, host)
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
