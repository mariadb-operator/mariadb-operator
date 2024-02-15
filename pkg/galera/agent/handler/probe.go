package handler

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Probe struct {
	mariadbKey     types.NamespacedName
	k8sClient      ctrlclient.Client
	refResolver    *refresolver.RefResolver
	responseWriter *mdbhttp.ResponseWriter
	logger         *logr.Logger
}

func NewProbe(mariadbKey types.NamespacedName, k8sClient ctrlclient.Client, responseWriter *mdbhttp.ResponseWriter,
	logger *logr.Logger) *Probe {
	return &Probe{
		mariadbKey:     mariadbKey,
		k8sClient:      k8sClient,
		refResolver:    refresolver.New(k8sClient),
		responseWriter: responseWriter,
		logger:         logger,
	}
}

func (p *Probe) Liveness(w http.ResponseWriter, r *http.Request) {
	mdb := p.getMariaDB(r.Context(), w)
	if mdb == nil {
		return
	}
	// avoid restarting Pods during cluster recovery
	if mdb.HasGaleraNotReadyCondition() {
		p.responseWriter.WriteOK(w, nil)
		return
	}

	sqlClient := p.getSqlClient(r.Context(), w, mdb)
	if sqlClient == nil {
		return
	}
	defer sqlClient.Close()

	status, err := sqlClient.GaleraClusterStatus(r.Context())
	if err != nil {
		p.logger.Error(err, "error getting cluster status")
		p.responseWriter.WriteError(w, "error getting cluster status")
		return
	}
	if status != "Primary" {
		p.logger.Error(errors.New("MariaDB Galera is unhealthy"), "status", status)
		p.responseWriter.WriteErrorf(w, "MariaDB Galera is unhealthy. Status: %s", status)
		return
	}

	p.responseWriter.WriteOK(w, nil)
}

func (p *Probe) Readiness(w http.ResponseWriter, r *http.Request) {
	mdb := p.getMariaDB(r.Context(), w)
	if mdb == nil {
		return
	}
	// keep sending traffic Pods during cluster recovery
	if mdb.HasGaleraNotReadyCondition() {
		p.responseWriter.WriteOK(w, nil)
		return
	}

	sqlClient := p.getSqlClient(r.Context(), w, mdb)
	if sqlClient == nil {
		return
	}
	defer sqlClient.Close()

	status, err := sqlClient.GaleraClusterStatus(r.Context())
	if err != nil {
		p.logger.Error(err, "error getting cluster status")
		p.responseWriter.WriteError(w, "error getting cluster status")
		return
	}
	if status != "Primary" {
		p.logger.Error(errors.New("MariaDB Galera is unhealthy"), "status", status)
		p.responseWriter.WriteErrorf(w, "MariaDB Galera is unhealthy. Status: %s", status)
		return
	}

	state, err := sqlClient.GaleraLocalState(r.Context())
	if err != nil {
		p.logger.Error(err, "error getting local state")
		p.responseWriter.WriteError(w, "error getting local state")
		return
	}
	if state != "Synced" {
		p.logger.Error(errors.New("MariaDB Galera is not synced"), "state", state)
		p.responseWriter.WriteErrorf(w, "MariaDB Galera is not synced. State: %s", state)
		return
	}

	p.responseWriter.WriteOK(w, nil)
}

func (p *Probe) getMariaDB(ctx context.Context, w http.ResponseWriter) *mariadbv1alpha1.MariaDB {
	var mdb mariadbv1alpha1.MariaDB
	if err := p.k8sClient.Get(ctx, p.mariadbKey, &mdb); err != nil {
		p.logger.Error(err, "error getting MariaDB")
		p.responseWriter.WriteError(w, "error getting MariaDB")
		return nil
	}
	return &mdb
}

func (p *Probe) getSqlClient(ctx context.Context, w http.ResponseWriter,
	mdb *mariadbv1alpha1.MariaDB) *sql.Client {
	env := "POD_NAME"
	podName := os.Getenv(env)
	if podName == "" {
		p.logger.Error(errors.New("error getting Pod name"), "Environment variable not found", "env", env)
		p.responseWriter.WriteErrorf(w, "error getting Pod name: Environment variable not found: %s", env)
		return nil
	}
	podIndex, err := statefulset.PodIndex(podName)
	if err != nil {
		p.logger.Error(err, "error Pod index")
		p.responseWriter.WriteError(w, "error Pod index")
		return nil
	}

	client, err := sql.NewInternalClientWithPodIndex(
		ctx,
		mdb,
		p.refResolver,
		*podIndex,
		sql.WithTimeout(5*time.Second),
	)
	if err != nil {
		p.logger.Error(err, "error getting SQL client")
		p.responseWriter.WriteError(w, "error getting SQL client")
		return nil
	}
	return client
}
