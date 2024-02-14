package handler

import (
	"sync"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/responsewriter"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
)

type Handler struct {
	Bootstrap   *Bootstrap
	GaleraState *GaleraState
	Recovery    *Recovery
}

func NewHandler(fileManager *filemanager.FileManager, logger *logr.Logger, recoveryOpts ...RecoveryOption) *Handler {
	mux := &sync.RWMutex{}
	bootstrapLogger := logger.WithName("bootstrap")
	galeraStateLogger := logger.WithName("galerastate")
	recoveryLogger := logger.WithName("recovery")

	bootstrap := NewBootstrap(
		fileManager,
		responsewriter.NewResponseWriter(&bootstrapLogger),
		mux,
		&bootstrapLogger,
	)
	galerastate := NewGaleraState(
		fileManager,
		responsewriter.NewResponseWriter(&galeraStateLogger),
		mux.RLocker(),
		&galeraStateLogger,
	)
	recovery := NewRecover(
		fileManager,
		responsewriter.NewResponseWriter(&recoveryLogger),
		mux,
		&recoveryLogger,
		recoveryOpts...,
	)

	return &Handler{
		Bootstrap:   bootstrap,
		GaleraState: galerastate,
		Recovery:    recovery,
	}
}
