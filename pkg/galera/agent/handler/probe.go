package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	galeraclient "github.com/mariadb-operator/mariadb-operator/pkg/galera/client"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/sql"
	sqlClientSet "github.com/mariadb-operator/mariadb-operator/pkg/sqlset"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Probe struct {
	mariadbKey      types.NamespacedName
	k8sClient       ctrlclient.Client
	refResolver     *refresolver.RefResolver
	responseWriter  *mdbhttp.ResponseWriter
	livenessLogger  logr.Logger
	readinessLogger logr.Logger
}

func NewProbe(mariadbKey types.NamespacedName, k8sClient ctrlclient.Client, responseWriter *mdbhttp.ResponseWriter,
	logger *logr.Logger) *Probe {
	return &Probe{
		mariadbKey:      mariadbKey,
		k8sClient:       k8sClient,
		refResolver:     refresolver.New(k8sClient),
		responseWriter:  responseWriter,
		livenessLogger:  logger.WithName("liveness"),
		readinessLogger: logger.WithName("readiness"),
	}
}

func (p *Probe) Liveness(w http.ResponseWriter, r *http.Request) {
	p.livenessLogger.V(1).Info("Probe started")
	var mdb mariadbv1alpha1.MariaDB
	if err := p.k8sClient.Get(r.Context(), p.mariadbKey, &mdb); err != nil {
		p.livenessLogger.Error(err, "error getting MariaDB")
		p.responseWriter.WriteError(w, "error getting MariaDB")
		return
	}

	// avoid restarting Pods during cluster recovery
	if mdb.HasGaleraNotReadyCondition() {
		p.livenessLogger.Info("Galera not ready. Returning OK")
		p.responseWriter.WriteOK(w, nil)
		return
	}

	sqlClientSet := sqlClientSet.NewClientSet(&mdb, p.refResolver)
	defer sqlClientSet.Close()
	galeraClient := galeraclient.NewGaleraClient(sqlClientSet, sql.WithTimeout(5*time.Second))

	podIndex, err := getPodIndex(r.Context(), &mdb)
	if err != nil {
		p.livenessLogger.Error(err, "error getting Pod index")
		p.responseWriter.WriteError(w, "error getting Pod index")
		return
	}

	healthy, err := galeraClient.IsPodHealthy(r.Context(), *podIndex)
	if err != nil {
		p.livenessLogger.Error(err, "error getting Pod health")
		p.responseWriter.WriteError(w, "error getting Pod health")
		return
	}
	if !healthy {
		p.livenessLogger.Error(err, "Pod not healthy")
		p.responseWriter.WriteError(w, "Pod not healthy")
		return
	}

	p.responseWriter.WriteOK(w, nil)
}

func (p *Probe) Readiness(w http.ResponseWriter, r *http.Request) {
	p.readinessLogger.V(1).Info("Probe started")
	var mdb mariadbv1alpha1.MariaDB
	if err := p.k8sClient.Get(r.Context(), p.mariadbKey, &mdb); err != nil {
		p.readinessLogger.Error(err, "error getting MariaDB")
		p.responseWriter.WriteError(w, "error getting MariaDB")
		return
	}

	// keep sending traffic to Pods during cluster recovery
	if mdb.HasGaleraNotReadyCondition() {
		p.readinessLogger.Info("Galera not ready. Returning OK")
		p.responseWriter.WriteOK(w, nil)
		return
	}

	sqlClientSet := sqlClientSet.NewClientSet(&mdb, p.refResolver)
	defer sqlClientSet.Close()
	galeraClient := galeraclient.NewGaleraClient(sqlClientSet, sql.WithTimeout(5*time.Second))

	podIndex, err := getPodIndex(r.Context(), &mdb)
	if err != nil {
		p.readinessLogger.Error(err, "error getting Pod index")
		p.responseWriter.WriteError(w, "error getting Pod index")
		return
	}

	synced, err := galeraClient.IsPodSynced(r.Context(), *podIndex)
	if err != nil {
		p.readinessLogger.Error(err, "error getting Pod sync")
		p.responseWriter.WriteError(w, "error getting Pod sync")
		return
	}
	if !synced {
		p.readinessLogger.Error(err, "Pod not synced")
		p.responseWriter.WriteError(w, "Pod not synced")
		return
	}

	p.responseWriter.WriteOK(w, nil)
}

func getPodIndex(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (*int, error) {
	env := "POD_NAME"
	podName := os.Getenv(env)
	if podName == "" {
		return nil, fmt.Errorf("environment variable '%s' not found", env)
	}
	podIndex, err := statefulset.PodIndex(podName)
	if err != nil {
		return nil, fmt.Errorf("error getting Pod index: %v", err)
	}
	return podIndex, nil
}
