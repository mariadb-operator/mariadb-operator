package replication

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ReplicationProbe struct {
	mariadbKey      types.NamespacedName
	k8sClient       ctrlclient.Client
	env             *environment.PodEnvironment
	responseWriter  *mdbhttp.ResponseWriter
	livenessLogger  logr.Logger
	readinessLogger logr.Logger
}

var requestTimeout = 3 * time.Second

func NewReplicationProbe(env *environment.PodEnvironment, k8sClient ctrlclient.Client, responseWriter *mdbhttp.ResponseWriter,
	logger *logr.Logger) router.ProbeHandler {
	return &ReplicationProbe{
		mariadbKey: types.NamespacedName{
			Name:      env.MariadbName,
			Namespace: env.PodNamespace,
		},
		k8sClient:       k8sClient,
		env:             env,
		responseWriter:  responseWriter,
		livenessLogger:  logger.WithName("liveness"),
		readinessLogger: logger.WithName("readiness"),
	}
}

func (p *ReplicationProbe) Liveness(w http.ResponseWriter, r *http.Request) {
	p.livenessLogger.V(1).Info("Probe started")

	sqlCtx, sqlCancel := context.WithTimeout(context.Background(), requestTimeout)
	defer sqlCancel()

	sqlClient, err := sql.NewLocalClientWithPodEnv(sqlCtx, p.env, sql.WithTimeout(requestTimeout))
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
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-threads
	if isReplica {
		replicaIORunning, err := sqlClient.ReplicaSlaveIORunning(sqlCtx)
		if err != nil {
			p.livenessLogger.Error(err, "error checking replica IO thread")
			p.responseWriter.WriteErrorf(w, "error checking replica IO thread: %v", err)
			return
		}
		if !replicaIORunning {
			p.livenessLogger.Error(nil, "Replica IO thread not running")
			p.responseWriter.WriteError(w, "Replica IO thread not running")
			return
		}

		replicaSQLRunning, err := sqlClient.ReplicaSlaveSQLRunning(sqlCtx)
		if err != nil {
			p.livenessLogger.Error(err, "error checking replica SQL thread")
			p.responseWriter.WriteErrorf(w, "error checking replica SQL thread: %v", err)
			return
		}
		if !replicaSQLRunning {
			p.livenessLogger.Error(nil, "Replica SQL thread not running")
			p.responseWriter.WriteError(w, "Replica SQL thread not running")
			return
		}

		p.livenessLogger.V(1).Info("Replica thread running status",
			"Slave_IO_Running", replicaIORunning,
			"Slave_SQL_Running", replicaSQLRunning,
		)
		p.responseWriter.WriteOK(w, nil)
		return
	}

	isPrimary, err := sqlClient.IsReplicationPrimary(sqlCtx)
	if err != nil {
		p.livenessLogger.Error(err, "error checking primary")
		p.responseWriter.WriteErrorf(w, "error checking primary: %v", err)
		return
	}
	if !isPrimary {
		p.livenessLogger.Error(nil, "Primary not configured")
		p.responseWriter.WriteError(w, "Primary not configured")
		return
	}

	p.responseWriter.WriteOK(w, nil)
}

func (p *ReplicationProbe) Readiness(w http.ResponseWriter, r *http.Request) {
	p.readinessLogger.V(1).Info("Probe started")

	sqlCtx, sqlCancel := context.WithTimeout(context.Background(), requestTimeout)
	defer sqlCancel()

	k8sCtx, k8sCancel := context.WithTimeout(context.Background(), requestTimeout)
	defer k8sCancel()

	sqlClient, err := sql.NewLocalClientWithPodEnv(sqlCtx, p.env, sql.WithTimeout(requestTimeout))
	if err != nil {
		p.readinessLogger.Error(err, "error getting SQL client")
		p.responseWriter.WriteErrorf(w, "error getting SQL client: %v", err)
		return
	}
	defer sqlClient.Close()

	isReplica, err := sqlClient.IsReplicationReplica(sqlCtx)
	if err != nil {
		p.readinessLogger.Error(err, "error checking replica")
		p.responseWriter.WriteErrorf(w, "error checking replica: %v", err)
		return
	}
	if isReplica {
		secondsBehindMaster, err := sqlClient.ReplicaSecondsBehindMaster(sqlCtx)
		if err != nil {
			p.readinessLogger.Error(err, "error checking replica seconds behind master")
			p.responseWriter.WriteErrorf(w, "error checking replica seconds behind master: %v", err)
			return
		}
		maxLagSeconds := p.getMaxLagSeconds(k8sCtx)

		if secondsBehindMaster > maxLagSeconds {
			p.readinessLogger.Error(nil, "Replica is lagging behind master", "seconds", secondsBehindMaster, "max-seconds", maxLagSeconds)
			p.responseWriter.WriteErrorf(w, "Replica is lagging %d seconds behind master (max seconds: %d)", secondsBehindMaster, maxLagSeconds)
			return
		}

		p.readinessLogger.V(1).Info("Replica lag status", "seconds", secondsBehindMaster)
		p.responseWriter.WriteOK(w, nil)
		return
	}

	isPrimary, err := sqlClient.IsReplicationPrimary(sqlCtx)
	if err != nil {
		p.readinessLogger.Error(err, "error checking primary")
		p.responseWriter.WriteErrorf(w, "error checking primary: %v", err)
		return
	}
	if !isPrimary {
		p.readinessLogger.Error(nil, "Primary not configured")
		p.responseWriter.WriteError(w, "Primary not configured")
		return
	}

	p.responseWriter.WriteOK(w, nil)
}

func (p *ReplicationProbe) getMaxLagSeconds(ctx context.Context) int {
	var mdb mariadbv1alpha1.MariaDB
	if err := p.k8sClient.Get(ctx, p.mariadbKey, &mdb); err != nil {
		p.readinessLogger.Error(err, "error getting MariaDB. Using default max replication lag")
		return 0
	}

	replication := ptr.Deref(mdb.Spec.Replication, mariadbv1alpha1.Replication{})
	replica := replication.Replica
	return ptr.Deref(replica.MaxLagSeconds, 0)
}
