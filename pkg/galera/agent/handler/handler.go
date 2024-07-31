package handler

import (
	"sync"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/state"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	Galera *Galera
	Probe  *Probe
}

func NewHandler(mariadbKey types.NamespacedName, client ctrlclient.Client, fileManager *filemanager.FileManager,
	state *state.State, logger *logr.Logger) *Handler {
	mux := &sync.RWMutex{}
	galeraLogger := logger.WithName("galera")
	probeLogger := logger.WithName("probe")

	galera := NewGalera(
		fileManager,
		state,
		mdbhttp.NewResponseWriter(&galeraLogger),
		mux,
		&galeraLogger,
	)
	probe := NewProbe(
		mariadbKey,
		client,
		mdbhttp.NewResponseWriter(&probeLogger),
		&probeLogger,
	)

	return &Handler{
		Galera: galera,
		Probe:  probe,
	}
}
