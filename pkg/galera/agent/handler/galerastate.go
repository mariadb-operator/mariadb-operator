package handler

import (
	"net/http"
	"os"
	"sync"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/agent/pkg/galera"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/responsewriter"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
)

type GaleraState struct {
	fileManager    *filemanager.FileManager
	responseWriter *responsewriter.ResponseWriter
	locker         sync.Locker
	logger         *logr.Logger
}

func NewGaleraState(fileManager *filemanager.FileManager, responseWriter *responsewriter.ResponseWriter, locker sync.Locker,
	logger *logr.Logger) *GaleraState {
	return &GaleraState{
		fileManager:    fileManager,
		responseWriter: responseWriter,
		locker:         locker,
		logger:         logger,
	}
}

func (g *GaleraState) Get(w http.ResponseWriter, r *http.Request) {
	g.locker.Lock()
	defer g.locker.Unlock()
	g.logger.V(1).Info("getting galera state")

	bytes, err := g.fileManager.ReadStateFile(galera.GaleraStateFileName)
	if err != nil {
		if os.IsNotExist(err) {
			g.responseWriter.Write(w, errors.NewAPIError("galera state not found"), http.StatusNotFound)
			return
		}
		g.responseWriter.WriteErrorf(w, "error reading galera state: %v", err)
		return
	}

	var galeraState galera.GaleraState
	if err := galeraState.Unmarshal(bytes); err != nil {
		g.responseWriter.WriteErrorf(w, "error unmarshaling galera state: %v", err)
		return
	}
	g.responseWriter.WriteOK(w, galeraState)
}
