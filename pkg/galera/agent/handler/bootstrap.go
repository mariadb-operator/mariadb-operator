package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/agent/pkg/galera"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/responsewriter"
	galeraErrors "github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
)

type Bootstrap struct {
	fileManager    *filemanager.FileManager
	responseWriter *responsewriter.ResponseWriter
	locker         sync.Locker
	logger         *logr.Logger
}

func NewBootstrap(fileManager *filemanager.FileManager, responseWriter *responsewriter.ResponseWriter, locker sync.Locker,
	logger *logr.Logger) *Bootstrap {
	return &Bootstrap{
		fileManager:    fileManager,
		responseWriter: responseWriter,
		locker:         locker,
		logger:         logger,
	}
}

func (b *Bootstrap) Put(w http.ResponseWriter, r *http.Request) {
	var bootstrap galera.Bootstrap
	if err := json.NewDecoder(r.Body).Decode(&bootstrap); err != nil {
		b.responseWriter.Write(w, galeraErrors.NewAPIErrorf("error decoding bootstrap: %v", err), http.StatusBadRequest)
		return
	}
	if err := bootstrap.Validate(); err != nil {
		b.responseWriter.Write(w, galeraErrors.NewAPIErrorf("invalid bootstrap: %v", err), http.StatusBadRequest)
		return
	}
	b.locker.Lock()
	defer b.locker.Unlock()
	b.logger.V(1).Info("enabling bootstrap")

	if err := b.fileManager.DeleteConfigFile(galera.RecoveryFileName); err != nil && !os.IsNotExist(err) {
		b.responseWriter.WriteErrorf(w, "error deleting existing recovery config: %v", err)
		return
	}

	if err := b.setSafeToBootstrap(&bootstrap); err != nil {
		b.responseWriter.WriteErrorf(w, "error setting safe to bootstrap: %v", err)
		return
	}

	if err := b.fileManager.WriteConfigFile(galera.BootstrapFileName, []byte(galera.BootstrapFile)); err != nil {
		b.responseWriter.WriteErrorf(w, "error writing bootstrap config: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Bootstrap) Delete(w http.ResponseWriter, r *http.Request) {
	b.locker.Lock()
	defer b.locker.Unlock()
	b.logger.V(1).Info("disabling bootstrap")

	if err := b.fileManager.DeleteConfigFile(galera.BootstrapFileName); err != nil {
		if os.IsNotExist(err) {
			b.responseWriter.Write(w, galeraErrors.NewAPIError("bootstrap config not found"), http.StatusNotFound)
			return
		}
		b.responseWriter.WriteErrorf(w, "error deleting bootstrap config: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Bootstrap) setSafeToBootstrap(bootstrap *galera.Bootstrap) error {
	bytes, err := b.fileManager.ReadStateFile(galera.GaleraStateFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("galera state does not exist")
		}
		return fmt.Errorf("error reading galera state: %v", err)
	}

	var galeraState galera.GaleraState
	if err := galeraState.Unmarshal(bytes); err != nil {
		return fmt.Errorf("error unmarshaling galera state: %v", err)
	}

	galeraState.UUID = bootstrap.UUID
	galeraState.Seqno = bootstrap.Seqno
	galeraState.SafeToBootstrap = true
	bytes, err = galeraState.Marshal()
	if err != nil {
		return fmt.Errorf("error marshaling galera state: %v", err)
	}

	if err := b.fileManager.WriteStateFile(galera.GaleraStateFileName, bytes); err != nil {
		return fmt.Errorf("error writing galera state: %v", err)
	}
	return nil
}
