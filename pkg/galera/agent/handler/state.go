package handler

import (
	"net/http"
	"os"
	"sync"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/state"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type State struct {
	fileManager    *filemanager.FileManager
	state          *state.State
	responseWriter *mdbhttp.ResponseWriter
	locker         sync.Locker
	logger         *logr.Logger
}

func NewState(fileManager *filemanager.FileManager, state *state.State, responseWriter *mdbhttp.ResponseWriter,
	locker sync.Locker, logger *logr.Logger) *State {
	return &State{
		fileManager:    fileManager,
		state:          state,
		responseWriter: responseWriter,
		locker:         locker,
		logger:         logger,
	}
}

func (g *State) GetGaleraState(w http.ResponseWriter, r *http.Request) {
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
