package handler

import (
	"net/http"
	"os"
	"sync"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type GaleraState struct {
	fileManager    *filemanager.FileManager
	responseWriter *mdbhttp.ResponseWriter
	locker         sync.Locker
	logger         *logr.Logger
}

func NewGaleraState(fileManager *filemanager.FileManager, responseWriter *mdbhttp.ResponseWriter, locker sync.Locker,
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

	bytes, err := g.fileManager.ReadStateFile(recovery.GaleraStateFileName)
	if err != nil {
		if os.IsNotExist(err) {
			g.responseWriter.Write(w, errors.NewAPIError("galera state not found"), http.StatusNotFound)
			return
		}
		g.responseWriter.WriteErrorf(w, "error reading galera state: %v", err)
		return
	}

	var galeraState recovery.GaleraState
	if err := galeraState.Unmarshal(bytes); err != nil {
		g.responseWriter.WriteErrorf(w, "error unmarshaling galera state: %v", err)
		return
	}
	g.responseWriter.WriteOK(w, galeraState)
}
