package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/go-logr/logr"
	galeraErrors "github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/state"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type Galera struct {
	fileManager    *filemanager.FileManager
	responseWriter *mdbhttp.ResponseWriter
	locker         sync.Locker
	logger         *logr.Logger
}

func NewGalera(fileManager *filemanager.FileManager, responseWriter *mdbhttp.ResponseWriter, locker sync.Locker,
	logger *logr.Logger) *Galera {
	return &Galera{
		fileManager:    fileManager,
		responseWriter: responseWriter,
		locker:         locker,
		logger:         logger,
	}
}

func (g *Galera) GetState(w http.ResponseWriter, r *http.Request) {
	g.locker.Lock()
	defer g.locker.Unlock()
	g.logger.V(1).Info("getting galera state")

	bytes, err := g.fileManager.ReadStateFile(state.GaleraStateFileName)
	if err != nil {
		if os.IsNotExist(err) {
			g.responseWriter.Write(w, http.StatusNotFound, galeraErrors.NewAPIError("galera state not found"))
			return
		}
		g.responseWriter.WriteErrorf(w, "error reading galera state: %v", err)
		return
	}
	if len(bytes) == 0 {
		g.responseWriter.Write(w, http.StatusNotFound, galeraErrors.NewAPIError("galera state empty"))
		return
	}

	var galeraState recovery.GaleraState
	if err := galeraState.Unmarshal(bytes); err != nil {
		g.responseWriter.WriteErrorf(w, "error unmarshalling galera state: %v", err)
		return
	}
	g.responseWriter.WriteOK(w, galeraState)
}

func (g *Galera) IsBootstrapEnabled(w http.ResponseWriter, r *http.Request) {
	exists, err := g.fileManager.ConfigFileExists(recovery.BootstrapFileName)
	if err != nil {
		g.responseWriter.WriteErrorf(w, "error checking bootstrap config: %v", err)
		return
	}
	if exists {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func (g *Galera) EnableBootstrap(w http.ResponseWriter, r *http.Request) {
	bootstrap, err := g.decodeAndValidateBootstrap(r)
	if err != nil {
		g.responseWriter.Write(w, http.StatusBadRequest, err)
		return
	}

	g.locker.Lock()
	defer g.locker.Unlock()
	g.logger.V(1).Info("enabling bootstrap")

	if err := g.setSafeToBootstrap(bootstrap); err != nil {
		g.responseWriter.WriteErrorf(w, "error setting safe to bootstrap: %v", err)
		return
	}

	if err := g.fileManager.WriteConfigFile(recovery.BootstrapFileName, []byte(recovery.BootstrapFile)); err != nil {
		g.responseWriter.WriteErrorf(w, "error writing bootstrap config: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (g *Galera) DisableBootstrap(w http.ResponseWriter, r *http.Request) {
	g.locker.Lock()
	defer g.locker.Unlock()
	g.logger.V(1).Info("disabling bootstrap")

	if err := g.fileManager.DeleteConfigFile(recovery.BootstrapFileName); err != nil {
		if os.IsNotExist(err) {
			g.responseWriter.Write(w, http.StatusNotFound, galeraErrors.NewAPIError("bootstrap config not found"))
			return
		}
		g.responseWriter.WriteErrorf(w, "error deleting bootstrap config: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (g *Galera) decodeAndValidateBootstrap(r *http.Request) (*recovery.Bootstrap, error) {
	if r.Body == nil || r.ContentLength <= 0 {
		return nil, nil
	}
	defer r.Body.Close()

	var bootstrap recovery.Bootstrap
	if err := json.NewDecoder(r.Body).Decode(&bootstrap); err != nil {
		return nil, galeraErrors.NewAPIErrorf("error decoding bootstrap: %v", err)
	}

	if err := bootstrap.Validate(); err != nil {
		return nil, galeraErrors.NewAPIErrorf("invalid bootstrap: %v", err)
	}
	return &bootstrap, nil
}

func (g *Galera) setSafeToBootstrap(bootstrap *recovery.Bootstrap) error {
	bytes, err := g.fileManager.ReadStateFile(state.GaleraStateFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("galera state does not exist")
		}
		return fmt.Errorf("error reading galera state: %v", err)
	}

	var galeraState recovery.GaleraState
	if err := galeraState.Unmarshal(bytes); err != nil {
		return fmt.Errorf("error unmarshalling galera state: %v", err)
	}

	galeraState.SafeToBootstrap = true
	if bootstrap != nil {
		galeraState.UUID = bootstrap.UUID
		galeraState.Seqno = bootstrap.Seqno
	}
	bytes, err = galeraState.Marshal()
	if err != nil {
		return fmt.Errorf("error marshaling galera state: %v", err)
	}

	if err := g.fileManager.WriteStateFile(state.GaleraStateFileName, bytes); err != nil {
		return fmt.Errorf("error writing galera state: %v", err)
	}
	return nil
}
