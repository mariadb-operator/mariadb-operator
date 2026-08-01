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
	// BinlogFile and BinlogPosition are the binary log coordinates recorded alongside the GTID.
	// They allow the caller to re-derive the GTID via BINLOG_GTID_POS when the recorded GTID
	// is inconsistent with the primary binary log (e.g. poisoned by a stale gtid_slave_pos).
	BinlogFile     string `json:"binlogFile,omitempty"`
	BinlogPosition uint64 `json:"binlogPosition,omitempty"`
}

func (h *ReplicationHandler) GetGtid(w http.ResponseWriter, r *http.Request) {
	h.logger.V(1).Info("getting GTID")

	bytes, err := h.fileManager.ReadStateFile(replication.MariaDBOperatorFileName)
	if err != nil {
		h.responseWriter.WriteErrorf(w, "error reading GTID file '%s': %v", replication.MariaDBOperatorFileName, err)
		return
	}
	meta, err := replication.ParseMetaFile(bytes)
	if err != nil {
		h.responseWriter.WriteErrorf(w, "error parsing GTID: %v", err)
		return
	}

	h.responseWriter.WriteOK(w, GtidResponse{
		Gtid:           meta.Gtid,
		BinlogFile:     meta.BinlogFile,
		BinlogPosition: meta.BinlogPosition,
	})
}
