package handler

import (
	"sync"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/state"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	Bootstrap *Bootstrap
	State     *State
	Probe     *Probe
}

func NewHandler(mariadbKey types.NamespacedName, client ctrlclient.Client, fileManager *filemanager.FileManager,
	initState *state.State, logger *logr.Logger) *Handler {
	mux := &sync.RWMutex{}
	bootstrapLogger := logger.WithName("bootstrap")
	stateLogger := logger.WithName("state")
	probeLogger := logger.WithName("probe")

	bootstrap := NewBootstrap(
		fileManager,
		mdbhttp.NewResponseWriter(&bootstrapLogger),
		mux,
		&bootstrapLogger,
	)
	state := NewState(
		fileManager,
		initState,
		mdbhttp.NewResponseWriter(&stateLogger),
		mux.RLocker(),
		&stateLogger,
	)
	probe := NewProbe(
		mariadbKey,
		client,
		mdbhttp.NewResponseWriter(&probeLogger),
		&probeLogger,
	)

	return &Handler{
		Bootstrap: bootstrap,
		State:     state,
		Probe:     probe,
	}
}
