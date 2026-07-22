package replication

import (
	"net/http"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/filemanager"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/gtid"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
)

// TODO: deprecate in favor of handler/gtid
// With the introduction of PITR, the GTID endpoint should not be tied to replication topology.
// This is kept for backwards compatibility purposes.
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

	bytes, err := h.fileManager.ReadStateFile(gtid.MariaDBOperatorFileName)
	if err != nil {
		h.responseWriter.WriteErrorf(w, "error reading GTID file '%s': %v", gtid.MariaDBOperatorFileName, err)
		return
	}
	g, err := gtid.ParseRawGtidInMetaFile(bytes)
	if err != nil {
		h.responseWriter.WriteErrorf(w, "error parsing GTID: %v", err)
		return
	}

	h.responseWriter.WriteOK(w, GtidResponse{
		Gtid: g,
	})
}
