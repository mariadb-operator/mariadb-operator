package standalone

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
)

// StandaloneProbe checks that the local mariadbd instance is able to serve queries.
type StandaloneProbe struct {
	env             *environment.PodEnvironment
	responseWriter  *mdbhttp.ResponseWriter
	livenessLogger  logr.Logger
	readinessLogger logr.Logger
}

var requestTimeout = 3 * time.Second

func NewStandaloneProbe(env *environment.PodEnvironment, responseWriter *mdbhttp.ResponseWriter,
	logger *logr.Logger) router.ProbeHandler {
	return &StandaloneProbe{
		env:             env,
		responseWriter:  responseWriter,
		livenessLogger:  logger.WithName("liveness"),
		readinessLogger: logger.WithName("readiness"),
	}
}

func (p *StandaloneProbe) Liveness(w http.ResponseWriter, r *http.Request) {
	p.livenessLogger.V(1).Info("Liveness Probe started")
	p.probe(w, p.livenessLogger)
}

func (p *StandaloneProbe) Readiness(w http.ResponseWriter, r *http.Request) {
	p.readinessLogger.V(1).Info("Readiness Probe started")
	p.probe(w, p.readinessLogger)
}

// probe is used for both Liveness and Readiness.
//
// It is the agent-based equivalent of the default 'SELECT 1' exec probe, with the advantage
// that the root password held by the agent can be updated at runtime without restarting the Pod.
func (p *StandaloneProbe) probe(w http.ResponseWriter, logger logr.Logger) {
	sqlCtx, sqlCancel := context.WithTimeout(context.Background(), requestTimeout)
	defer sqlCancel()

	sqlClient, err := sql.NewLocalClientWithPodEnv(sqlCtx, p.env, sql.WithTimeout(requestTimeout))
	if err != nil {
		logger.Error(err, "error getting SQL client")
		p.responseWriter.WriteErrorf(w, "error getting SQL client: %v", err)
		return
	}
	defer sqlClient.Close()

	if err := sqlClient.Exec(sqlCtx, "SELECT 1"); err != nil {
		logger.Error(err, "error querying MariaDB")
		p.responseWriter.WriteErrorf(w, "error querying MariaDB: %v", err)
		return
	}

	p.responseWriter.WriteOK(w, nil)
}
