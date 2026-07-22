package replication

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	// "html/template"
	"strconv"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/secret"
	env "github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReplicationConfigClient struct {
	client.Client
	builder          *builder.Builder
	refResolver      *refresolver.RefResolver
	secretReconciler *secret.SecretReconciler
}

func NewReplicationConfigClient(client client.Client, builder *builder.Builder,
	secretReconciler *secret.SecretReconciler) *ReplicationConfigClient {
	return &ReplicationConfigClient{
		Client:           client,
		builder:          builder,
		refResolver:      refresolver.New(client),
		secretReconciler: secretReconciler,
	}
}

func NewReplicationConfig(env *env.PodEnvironment) ([]byte, error) {
	var sId int

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
	externalReplEnabled, err := env.IsExternalReplEnabled()
	if err != nil {
		return nil, fmt.Errorf("error checking if external replication is enabled: %v", err)
	}

	externalReplServerIdOffset, err := env.ExternalReplServerIdOffset()
	if err != nil {
		return nil, fmt.Errorf("error get serverId offset for external replication: %v", err)
	}

	if externalReplEnabled && externalReplServerIdOffset != nil {
		sId, err = offsetServerId(env.PodName, *externalReplServerIdOffset)
		if err != nil {
			return nil, fmt.Errorf("error getting server_id with offset server ID: %v", err)
		}
	} else {
		serverIDStartIndex, err := serverIDStartIndex(env.MariaDBReplServerIDStartIndex)
		if err != nil {
			return nil, fmt.Errorf("error getting server ID start index: %v", err)
		}
		sId, err = serverId(env.PodName, serverIDStartIndex)
		if err != nil {
			return nil, fmt.Errorf("error getting server ID: %v", err)
		}
	}

	syncBinlog, err := env.ReplSyncBinlog()
	if err != nil {
		return nil, fmt.Errorf("error getting master sync binlog: %v", err)
	}

	filteredTables := env.ExternalReplFilteredTables()

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
{{- range .ReplicateDoTables }}
replicate_do_table={{ . }}
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
		ReplicateDoTables       []string
	}{
		LogName:                 env.MariadbName,
		GtidStrictMode:          gtidStrictMode,
		GtidDomainID:            gtidDomainID,
		SemiSyncEnabled:         semiSyncEnabled,
		SemiSyncMasterTimeout:   semiSyncMasterTimeout,
		SemiSyncMasterWaitPoint: env.MariaDBReplSemiSyncMasterWaitPoint,
		ServerID:                sId,
		SyncBinlog:              syncBinlog,
		ReplicateDoTables:       filteredTables,
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

func externalReplPasswordRef(mariadb *mariadbv1alpha1.MariaDB, r *refresolver.RefResolver,
	ctx context.Context) (mariadbv1alpha1.SecretKeySelector, error) {
	replication := mariadb.Replication()
	// if mariadb.Replication().Enabled && mariadb.Replication().Replica.ReplPasswordSecretKeyRef != nil {
	// 	return mariadb.Replication().Replica.ReplPasswordSecretKeyRef.SecretKeySelector, nil
	// }
	if replication.IsExternalReplication() {
		emdbRef := replication.GetExternalReplicationRef()
		emdb, err := r.ExternalMariaDB(ctx, &emdbRef, mariadb.Namespace)
		if err == nil {
			return *emdb.GetSUCredential(), nil
		}
	}
	return mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: "",
		},
		Key: "",
	}, fmt.Errorf("not able to get PasswordRef for external replication")
}

func offsetServerId(podName string, offset int) (int, error) {
	podIndex, err := statefulset.PodIndex(podName)
	if err != nil {
		return 0, fmt.Errorf("error getting Pod index: %v", err)
	}
	return *podIndex + offset, nil
}

func formatAccountName(username, host string) string {
	return fmt.Sprintf("'%s'@'%s'", username, host)
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
