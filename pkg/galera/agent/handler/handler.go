package handler

import (
	"sync"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	Bootstrap   *Bootstrap
	GaleraState *GaleraState
	Recovery    *Recovery
	Probe       *Probe
}

func NewHandler(mariadbKey types.NamespacedName, client ctrlclient.Client, fileManager *filemanager.FileManager,
	logger *logr.Logger, recoveryOpts ...RecoveryOption) *Handler {
	mux := &sync.RWMutex{}
	bootstrapLogger := logger.WithName("bootstrap")
	galeraStateLogger := logger.WithName("galerastate")
	recoveryLogger := logger.WithName("recovery")
	probeLogger := logger.WithName("probe")

	bootstrap := NewBootstrap(
		fileManager,
		mdbhttp.NewResponseWriter(&bootstrapLogger),
		mux,
		&bootstrapLogger,
	)
	galerastate := NewGaleraState(
		fileManager,
		mdbhttp.NewResponseWriter(&galeraStateLogger),
		mux.RLocker(),
		&galeraStateLogger,
	)
	recovery := NewRecover(
		fileManager,
		mdbhttp.NewResponseWriter(&recoveryLogger),
		mux,
		&recoveryLogger,
		recoveryOpts...,
	)
	probe := NewProbe(
		mariadbKey,
		client,
		mdbhttp.NewResponseWriter(&probeLogger),
		&probeLogger,
	)

	return &Handler{
		Bootstrap:   bootstrap,
		GaleraState: galerastate,
		Recovery:    recovery,
		Probe:       probe,
	}
}
