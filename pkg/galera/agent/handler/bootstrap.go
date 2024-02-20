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
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type Bootstrap struct {
	fileManager    *filemanager.FileManager
	responseWriter *mdbhttp.ResponseWriter
	locker         sync.Locker
	logger         *logr.Logger
}

func NewBootstrap(fileManager *filemanager.FileManager, responseWriter *mdbhttp.ResponseWriter, locker sync.Locker,
	logger *logr.Logger) *Bootstrap {
	return &Bootstrap{
		fileManager:    fileManager,
		responseWriter: responseWriter,
		locker:         locker,
		logger:         logger,
	}
}

func (b *Bootstrap) IsBootstrapEnabled(w http.ResponseWriter, r *http.Request) {
	exists, err := b.fileManager.ConfigFileExists(recovery.BootstrapFileName)
	if err != nil {
		b.responseWriter.WriteErrorf(w, "error checking bootstrap config: %v", err)
		return
	}
	if exists {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func (b *Bootstrap) Enable(w http.ResponseWriter, r *http.Request) {
	var bootstrap recovery.Bootstrap
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

	if err := b.fileManager.DeleteConfigFile(recovery.RecoveryFileName); err != nil && !os.IsNotExist(err) {
		b.responseWriter.WriteErrorf(w, "error deleting existing recovery config: %v", err)
		return
	}

	if err := b.setSafeToBootstrap(&bootstrap); err != nil {
		b.responseWriter.WriteErrorf(w, "error setting safe to bootstrap: %v", err)
		return
	}

	if err := b.fileManager.WriteConfigFile(recovery.BootstrapFileName, []byte(recovery.BootstrapFile)); err != nil {
		b.responseWriter.WriteErrorf(w, "error writing bootstrap config: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Bootstrap) Disable(w http.ResponseWriter, r *http.Request) {
	b.locker.Lock()
	defer b.locker.Unlock()
	b.logger.V(1).Info("disabling bootstrap")

	if err := b.fileManager.DeleteConfigFile(recovery.BootstrapFileName); err != nil {
		if os.IsNotExist(err) {
			b.responseWriter.Write(w, galeraErrors.NewAPIError("bootstrap config not found"), http.StatusNotFound)
			return
		}
		b.responseWriter.WriteErrorf(w, "error deleting bootstrap config: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Bootstrap) setSafeToBootstrap(bootstrap *recovery.Bootstrap) error {
	bytes, err := b.fileManager.ReadStateFile(recovery.GaleraStateFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("galera state does not exist")
		}
		return fmt.Errorf("error reading galera state: %v", err)
	}

	var galeraState recovery.GaleraState
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

	if err := b.fileManager.WriteStateFile(recovery.GaleraStateFileName, bytes); err != nil {
		return fmt.Errorf("error writing galera state: %v", err)
	}
	return nil
}
