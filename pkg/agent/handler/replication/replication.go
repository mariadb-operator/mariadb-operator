package replication

import (
	"net/http"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/filemanager"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
)

type ReplicationHandler struct {
	fileManager    *filemanager.FileManager
	responseWriter *mdbhttp.ResponseWriter
	logger         *logr.Logger
}

func NewReplicationHandler(fileManager *filemanager.FileManager, responseWriter *mdbhttp.ResponseWriter,
	logger *logr.Logger) router.RouteHandler {
	return &ReplicationHandler{
		fileManager:    fileManager,
		responseWriter: responseWriter,
		logger:         logger,
	}
}

func (h *ReplicationHandler) SetupRoutes(router *chi.Mux) {
	router.Route("/replication", func(r chi.Router) {
		r.Get("/gtid", h.GetGtid)
	})
}

type GtidResponse struct {
	Gtid string `json:"gtid"`
}

func (h *ReplicationHandler) GetGtid(w http.ResponseWriter, r *http.Request) {
	h.logger.V(1).Info("getting GTID")

	bytes, err := h.fileManager.ReadStateFile(replication.MariaDBOperatorFileName)
	if err != nil {
		h.responseWriter.WriteErrorf(w, "error reading GTID file '%s': %v", replication.MariaDBOperatorFileName, err)
		return
	}
	gtid, err := replication.ParseRawGtidInMetaFile(bytes)
	if err != nil {
		h.responseWriter.WriteErrorf(w, "error parsing GTID: %v", err)
		return
	}

	h.responseWriter.WriteOK(w, GtidResponse{
		Gtid: gtid,
	})
}
