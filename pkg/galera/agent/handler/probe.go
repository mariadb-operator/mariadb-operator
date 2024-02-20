package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	galeraclient "github.com/mariadb-operator/mariadb-operator/pkg/galera/client"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/pkg/sql"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Probe struct {
	mariadbKey      types.NamespacedName
	k8sClient       ctrlclient.Client
	responseWriter  *mdbhttp.ResponseWriter
	livenessLogger  logr.Logger
	readinessLogger logr.Logger
}

func NewProbe(mariadbKey types.NamespacedName, k8sClient ctrlclient.Client, responseWriter *mdbhttp.ResponseWriter,
	logger *logr.Logger) *Probe {
	return &Probe{
		mariadbKey:      mariadbKey,
		k8sClient:       k8sClient,
		responseWriter:  responseWriter,
		livenessLogger:  logger.WithName("liveness"),
		readinessLogger: logger.WithName("readiness"),
	}
}

func (p *Probe) Liveness(w http.ResponseWriter, r *http.Request) {
	p.livenessLogger.V(1).Info("Probe started")

	k8sCtx, k8sCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer k8sCancel()

	var mdb mariadbv1alpha1.MariaDB
	if err := p.k8sClient.Get(k8sCtx, p.mariadbKey, &mdb); err != nil {
		p.livenessLogger.Error(err, "error getting MariaDB")
	}

	if mdb.HasGaleraNotReadyCondition() {
		p.livenessLogger.Info("Galera not ready. Returning OK to facilitate recovery")
		p.responseWriter.WriteOK(w, nil)
		return
	}

	env, err := environment.GetPodEnv(context.Background())
	if err != nil {
		p.livenessLogger.Error(err, "error getting environment")
		p.responseWriter.WriteErrorf(w, "error getting environment: %v", err)
		return
	}

	sqlCtx, sqlCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer sqlCancel()

	sqlClient, err := sql.NewLocalClientWithPodEnv(sqlCtx, env, sql.WithTimeout(1*time.Second))
	if err != nil {
		p.livenessLogger.Error(err, "error getting SQL client")
		p.responseWriter.WriteErrorf(w, "error getting SQL client: %v", err)
		return
	}
	defer sqlClient.Close()

	healthy, err := galeraclient.IsPodHealthy(sqlCtx, sqlClient)
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

	k8sCtx, k8sCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer k8sCancel()

	var mdb mariadbv1alpha1.MariaDB
	if err := p.k8sClient.Get(k8sCtx, p.mariadbKey, &mdb); err != nil {
		p.readinessLogger.Error(err, "error getting MariaDB")
	}

	if mdb.HasGaleraNotReadyCondition() {
		p.readinessLogger.Info("Galera not ready. Returning OK to facilitate recovery")
		p.responseWriter.WriteOK(w, nil)
		return
	}

	env, err := environment.GetPodEnv(context.Background())
	if err != nil {
		p.readinessLogger.Error(err, "error getting environment")
		p.responseWriter.WriteErrorf(w, "error getting environment: %v", err)
		return
	}

	sqlCtx, sqlCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer sqlCancel()

	sqlClient, err := sql.NewLocalClientWithPodEnv(sqlCtx, env, sql.WithTimeout(1*time.Second))
	if err != nil {
		p.readinessLogger.Error(err, "error getting SQL client")
		p.responseWriter.WriteErrorf(w, "error getting SQL client: %v", err)
		return
	}
	defer sqlClient.Close()

	healthy, err := galeraclient.IsPodHealthy(sqlCtx, sqlClient)
	if err != nil {
		p.readinessLogger.Error(err, "error getting Pod health")
		p.responseWriter.WriteError(w, "error getting Pod health")
		return
	}
	if !healthy {
		p.readinessLogger.Error(err, "Pod not healthy")
		p.responseWriter.WriteError(w, "Pod not healthy")
		return
	}

	state, err := sqlClient.GaleraLocalState(sqlCtx)
	if err != nil {
		p.readinessLogger.Error(err, "error getting Pod state")
		p.responseWriter.WriteError(w, "error getting Pod state")
		return
	}
	if state == galeraclient.GaleraStateSynced {
		p.responseWriter.WriteOK(w, nil)
		return
	}

	galera := ptr.Deref(mdb.Spec.Galera, mariadbv1alpha1.Galera{})
	availableWhenDonor := ptr.Deref(galera.AvailableWhenDonor, false)

	if availableWhenDonor && state == galeraclient.GaleraStateDonor {
		p.responseWriter.WriteOK(w, nil)
		return
	}

	p.readinessLogger.Error(err, "Pod in non ready state", "state", state)
	p.responseWriter.WriteErrorf(w, "Pod in non ready state: %s", state)
}
