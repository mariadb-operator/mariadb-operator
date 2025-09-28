package galera

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	agenterrors "github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/errors"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/filemanager"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/galera/recovery"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/galera/state"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
)

type GaleraHandler struct {
	fileManager    *filemanager.FileManager
	responseWriter *mdbhttp.ResponseWriter
	logger         *logr.Logger
	mux            *sync.RWMutex
}

func NewGaleraHandler(fileManager *filemanager.FileManager, responseWriter *mdbhttp.ResponseWriter, logger *logr.Logger) router.RouteHandler {
	return &GaleraHandler{
		fileManager:    fileManager,
		responseWriter: responseWriter,
		logger:         logger,
		mux:            &sync.RWMutex{},
	}
}

func (g *GaleraHandler) SetupRoutes(router *chi.Mux) {
	router.Route("/galera", func(r chi.Router) {
		r.Get("/state", g.GetState)
		r.Route("/bootstrap", func(r chi.Router) {
			r.Get("/", g.IsBootstrapEnabled)
			r.Put("/", g.EnableBootstrap)
			r.Delete("/", g.DisableBootstrap)
		})
	})
}

func (g *GaleraHandler) GetState(w http.ResponseWriter, r *http.Request) {
	g.mux.Lock()
	defer g.mux.Unlock()
	g.logger.V(1).Info("getting galera state")

	bytes, err := g.fileManager.ReadStateFile(state.GaleraStateFileName)
	if err != nil {
		if os.IsNotExist(err) {
			g.responseWriter.Write(w, http.StatusNotFound, agenterrors.NewAPIError("galera state not found"))
			return
		}
		g.responseWriter.WriteErrorf(w, "error reading galera state: %v", err)
		return
	}
	if len(bytes) == 0 {
		g.responseWriter.Write(w, http.StatusNotFound, agenterrors.NewAPIError("galera state empty"))
		return
	}

	var galeraState recovery.GaleraState
	if err := galeraState.Unmarshal(bytes); err != nil {
		g.responseWriter.WriteErrorf(w, "error unmarshalling galera state: %v", err)
		return
	}
	g.responseWriter.WriteOK(w, galeraState)
}

func (g *GaleraHandler) IsBootstrapEnabled(w http.ResponseWriter, r *http.Request) {
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

func (g *GaleraHandler) EnableBootstrap(w http.ResponseWriter, r *http.Request) {
	bootstrap, err := g.decodeAndValidateBootstrap(r)
	if err != nil {
		g.responseWriter.Write(w, http.StatusBadRequest, err)
		return
	}

	g.mux.Lock()
	defer g.mux.Unlock()
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

func (g *GaleraHandler) DisableBootstrap(w http.ResponseWriter, r *http.Request) {
	g.mux.Lock()
	defer g.mux.Unlock()
	g.logger.V(1).Info("disabling bootstrap")

	if err := g.fileManager.DeleteConfigFile(recovery.BootstrapFileName); err != nil {
		if os.IsNotExist(err) {
			g.responseWriter.Write(w, http.StatusNotFound, agenterrors.NewAPIError("bootstrap config not found"))
			return
		}
		g.responseWriter.WriteErrorf(w, "error deleting bootstrap config: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (g *GaleraHandler) decodeAndValidateBootstrap(r *http.Request) (*recovery.Bootstrap, error) {
	if r.Body == nil || r.ContentLength <= 0 {
		return nil, nil
	}
	defer r.Body.Close()

	var bootstrap recovery.Bootstrap
	if err := json.NewDecoder(r.Body).Decode(&bootstrap); err != nil {
		return nil, agenterrors.NewAPIErrorf("error decoding bootstrap: %v", err)
	}

	if err := bootstrap.Validate(); err != nil {
		return nil, agenterrors.NewAPIErrorf("invalid bootstrap: %v", err)
	}
	return &bootstrap, nil
}

func (g *GaleraHandler) setSafeToBootstrap(bootstrap *recovery.Bootstrap) error {
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
