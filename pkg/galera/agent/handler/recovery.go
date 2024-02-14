package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/agent/pkg/galera"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/responsewriter"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Recovery struct {
	fileManager    *filemanager.FileManager
	responseWriter *responsewriter.ResponseWriter
	locker         sync.Locker
	logger         *logr.Logger
	timeout        time.Duration
}

type RecoveryOption func(*Recovery)

func WithRecoveryTimeout(timeout time.Duration) RecoveryOption {
	return func(r *Recovery) {
		r.timeout = timeout
	}
}

func NewRecover(fileManager *filemanager.FileManager, responseWriter *responsewriter.ResponseWriter, locker sync.Locker,
	logger *logr.Logger, opts ...RecoveryOption) *Recovery {
	recovery := &Recovery{
		fileManager:    fileManager,
		responseWriter: responseWriter,
		locker:         locker,
		logger:         logger,
		timeout:        1 * time.Minute,
	}
	for _, setOpts := range opts {
		setOpts(recovery)
	}
	return recovery
}

func (r *Recovery) Put(w http.ResponseWriter, req *http.Request) {
	r.locker.Lock()
	defer r.locker.Unlock()
	r.logger.V(1).Info("enabling recovery")

	if err := r.fileManager.DeleteConfigFile(recovery.BootstrapFileName); err != nil && !os.IsNotExist(err) {
		r.responseWriter.WriteErrorf(w, "error deleting existing bootstrap config: %v", err)
		return
	}

	if err := r.fileManager.DeleteStateFile(recovery.RecoveryLogFileName); err != nil && !os.IsNotExist(err) {
		r.responseWriter.WriteErrorf(w, "error deleting existing recovery log: %v", err)
		return
	}

	if err := r.fileManager.WriteConfigFile(recovery.RecoveryFileName, []byte(recovery.RecoveryFile)); err != nil {
		r.responseWriter.WriteErrorf(w, "error writing recovery config: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (r *Recovery) Post(w http.ResponseWriter, req *http.Request) {
	r.locker.Lock()
	defer r.locker.Unlock()
	r.logger.V(1).Info("starting recovery")

	exists, err := r.fileManager.ConfigFileExists(galera.RecoveryFileName)
	if err != nil {
		r.responseWriter.WriteErrorf(w, "error checking recovery config: %v", err)
		return
	}
	if !exists {
		r.responseWriter.Write(w, errors.NewAPIError("recovery config not found"), http.StatusNotFound)
		return
	}

	recoveryCtx, cancel := context.WithTimeout(req.Context(), r.timeout)
	defer cancel()

	bootstrap, err := r.pollUntilRecovered(recoveryCtx)
	if err != nil {
		r.responseWriter.WriteErrorf(w, "error recovering galera: %v", err)
		return
	}
	r.responseWriter.WriteOK(w, bootstrap)
}

func (r *Recovery) Delete(w http.ResponseWriter, req *http.Request) {
	r.locker.Lock()
	defer r.locker.Unlock()
	r.logger.V(1).Info("disabling recovery")

	if err := r.fileManager.DeleteConfigFile(galera.RecoveryFileName); err != nil {
		if os.IsNotExist(err) {
			r.responseWriter.Write(w, errors.NewAPIError("recovery config not found"), http.StatusNotFound)
			return
		}
		r.responseWriter.WriteErrorf(w, "error deleting recovery config: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (r *Recovery) pollUntilRecovered(ctx context.Context) (*galera.Bootstrap, error) {
	var bootstrap *galera.Bootstrap
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(context.Context) (bool, error) {
		b, err := r.recover()
		if err != nil {
			r.logger.Error(err, "error recovering galera from recovery log")
			return false, nil
		}
		bootstrap = b
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return bootstrap, nil
}

func (r *Recovery) recover() (*galera.Bootstrap, error) {
	bytes, err := r.fileManager.ReadStateFile(galera.RecoveryLogFileName)
	if err != nil {
		return nil, fmt.Errorf("error reading Galera state file: %v", err)
	}
	var bootstrap galera.Bootstrap
	if err := bootstrap.Unmarshal(bytes); err != nil {
		return nil, fmt.Errorf("error unmarshaling bootstrap: %v", err)
	}
	return &bootstrap, nil
}
