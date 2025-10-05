package replication

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/filemanager"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/replication"
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
	gtid, err := parseGtid(bytes)
	if err != nil {
		h.responseWriter.WriteErrorf(w, "error parsing GTID: %v", err)
		return
	}

	h.responseWriter.WriteOK(w, GtidResponse{
		Gtid: gtid,
	})
}

// parseGtid extracts the GTID from a mariadb-operator.info file.
// Example line: "mariadb-repl-bin.000001 335 0-10-9"
func parseGtid(fileBytes []byte) (string, error) {
	trimmed := bytes.TrimSpace(fileBytes)
	if len(trimmed) == 0 {
		return "", errors.New("file is empty")
	}
	parts := strings.Fields(string(trimmed))
	if len(parts) < 3 {
		return "", errors.New("unexpected file format, expected at least 3 fields")
	}
	return parts[2], nil
}
