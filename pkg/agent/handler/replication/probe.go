package replication

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
)

type ReplicationProbe struct {
	env             *environment.PodEnvironment
	responseWriter  *mdbhttp.ResponseWriter
	livenessLogger  logr.Logger
	readinessLogger logr.Logger
}

func NewReplicationProbe(env *environment.PodEnvironment, responseWriter *mdbhttp.ResponseWriter,
	logger *logr.Logger) router.ProbeHandler {
	return &ReplicationProbe{
		env:             env,
		responseWriter:  responseWriter,
		livenessLogger:  logger.WithName("liveness"),
		readinessLogger: logger.WithName("readiness"),
	}
}

func (p *ReplicationProbe) Liveness(w http.ResponseWriter, r *http.Request) {
	p.livenessLogger.V(1).Info("Probe started")

	sqlCtx, sqlCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer sqlCancel()

	sqlClient, err := sql.NewLocalClientWithPodEnv(sqlCtx, p.env, sql.WithTimeout(1*time.Second))
	if err != nil {
		p.livenessLogger.Error(err, "error getting SQL client")
		p.responseWriter.WriteErrorf(w, "error getting SQL client: %v", err)
		return
	}
	defer sqlClient.Close()

	isReplica, err := sqlClient.IsReplicationReplica(sqlCtx)
	if err != nil {
		p.livenessLogger.Error(err, "error checking replica")
		p.responseWriter.WriteErrorf(w, "error checking replica: %v", err)
		return
	}
	if isReplica {
		replicaIORunning, err := sqlClient.ReplicaSlaveIORunning(sqlCtx)
		if err != nil {
			p.livenessLogger.Error(err, "error checking replica IO thread")
			p.responseWriter.WriteErrorf(w, "error checking replica IO thread: %v", err)
			return
		}
		if !replicaIORunning {
			p.livenessLogger.Error(err, "Replica IO thread not running")
			p.responseWriter.WriteError(w, "Replica IO thread not running")
			return
		}

		p.livenessLogger.V(1).Info("Replica IO thread status", "running", replicaIORunning)
		p.responseWriter.WriteOK(w, nil)
		return
	}

	_, err = sqlClient.IsReplicationPrimary(sqlCtx)
	if err != nil {
		p.livenessLogger.Error(err, "error checking primary")
		p.responseWriter.WriteErrorf(w, "error checking primary: %v", err)
		return
	}
	// if isPrimary {
	// 	// TODO: check SHOW MASTER STATUS
	// }

	p.responseWriter.WriteOK(w, nil)
}

func (p *ReplicationProbe) Readiness(w http.ResponseWriter, r *http.Request) {
	p.readinessLogger.V(1).Info("Probe started")

	sqlCtx, sqlCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer sqlCancel()

	sqlClient, err := sql.NewLocalClientWithPodEnv(sqlCtx, p.env, sql.WithTimeout(1*time.Second))
	if err != nil {
		p.readinessLogger.Error(err, "error getting SQL client")
		p.responseWriter.WriteErrorf(w, "error getting SQL client: %v", err)
		return
	}
	defer sqlClient.Close()

	_, err = sqlClient.IsReplicationReplica(sqlCtx)
	if err != nil {
		p.readinessLogger.Error(err, "error checking replica")
		p.responseWriter.WriteErrorf(w, "error checking replica: %v", err)
		return
	}
	// if isReplica {
	// 	// TODO: check Seconds_Behind_Master
	// }

	_, err = sqlClient.IsReplicationPrimary(sqlCtx)
	if err != nil {
		p.readinessLogger.Error(err, "error checking primary")
		p.responseWriter.WriteErrorf(w, "error checking primary: %v", err)
		return
	}
	// if isPrimary {
	// 	// TODO: check SHOW MASTER STATUS
	// }

	p.responseWriter.WriteOK(w, nil)
}
