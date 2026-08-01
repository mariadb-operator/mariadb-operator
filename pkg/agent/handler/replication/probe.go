package replication

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
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

	mux               sync.Mutex
	ioConnectingSince *time.Time
}

var (
	requestTimeout = 3 * time.Second
	// ioConnectingGrace bounds how long a "Connecting" IO thread is tolerated by the liveness probe.
	// Transient reconnections (e.g. primary restarts) must not kill the replica, but an IO thread
	// stuck in "Connecting" indefinitely means replication is silently stalled and serving stale reads.
	ioConnectingGrace = 5 * time.Minute
)

// trackIOConnecting tracks how long the IO thread has been in "Connecting" state across probe calls.
// It returns true while the state is within the tolerated grace period.
func (p *ReplicationProbe) trackIOConnecting(now time.Time) bool {
	p.mux.Lock()
	defer p.mux.Unlock()
	if p.ioConnectingSince == nil {
		p.ioConnectingSince = &now
		return true
	}
	return now.Sub(*p.ioConnectingSince) <= ioConnectingGrace
}

func (p *ReplicationProbe) resetIOConnecting() {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.ioConnectingSince = nil
}

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
	if isReplica {
		status, err := sqlClient.ReplicaStatus(sqlCtx, p.livenessLogger)
		if err != nil {
			p.livenessLogger.Error(err, "error getting replica status")
			p.responseWriter.WriteErrorf(w, "error getting replica status: %v", err)
			return
		}

		if !status.IsIOThreadActive() {
			p.resetIOConnecting()
			p.livenessLogger.Error(nil, "Replica IO thread not running")
			p.responseWriter.WriteError(w, "Replica IO thread not running")
			return
		}
		if status.IsIOThreadRunning() {
			p.resetIOConnecting()
		} else if !p.trackIOConnecting(time.Now()) {
			p.livenessLogger.Error(nil, "Replica IO thread stuck in Connecting state", "grace", ioConnectingGrace)
			p.responseWriter.WriteErrorf(w, "Replica IO thread stuck in Connecting state for over %s", ioConnectingGrace)
			return
		}
		if !status.IsSQLThreadRunning() {
			p.livenessLogger.Error(nil, "Replica SQL thread not running")
			p.responseWriter.WriteError(w, "Replica SQL thread not running")
			return
		}

		p.livenessLogger.V(1).Info(
			"Replica thread running status",
			"Slave_IO_Running", status.IsIOThreadRunning(),
			"Slave_IO_State", ptr.Deref(status.SlaveIOState, ""),
			"Slave_SQL_Running", status.IsSQLThreadRunning(),
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
		status, err := sqlClient.ReplicaStatus(sqlCtx, p.readinessLogger)
		if err != nil {
			p.readinessLogger.Error(err, "error getting replica status")
			p.responseWriter.WriteErrorf(w, "error getting replica status: %v", err)
			return
		}
		if !status.IsIOThreadRunning() {
			p.readinessLogger.Error(nil, "Replica IO thread not running", "state", ptr.Deref(status.SlaveIOState, ""))
			p.responseWriter.WriteErrorf(w, "Replica IO thread not running (state: %s)", ptr.Deref(status.SlaveIOState, ""))
			return
		}
		if !status.IsSQLThreadRunning() {
			p.readinessLogger.Error(nil, "Replica SQL thread not running")
			p.responseWriter.WriteError(w, "Replica SQL thread not running")
			return
		}
		if status.SecondsBehindMaster == nil {
			p.readinessLogger.Error(nil, "could not determine replica lag")
			p.responseWriter.WriteError(w, "could not determine replica lag")
			return
		}
		secondsBehindMaster := *status.SecondsBehindMaster

		maxLagSeconds := p.getMaxLagSeconds(k8sCtx)
		if secondsBehindMaster > maxLagSeconds {
			p.readinessLogger.Error(nil, "Replica is lagging behind master", "seconds", secondsBehindMaster, "max-seconds", maxLagSeconds)
			p.responseWriter.WriteErrorf(w, "Replica is lagging %d seconds behind master (max seconds: %d)", secondsBehindMaster, maxLagSeconds)
			return
		}

		p.readinessLogger.V(1).Info(
			"Replica lag status",
			"seconds", secondsBehindMaster,
			"max-seconds", maxLagSeconds,
		)
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
