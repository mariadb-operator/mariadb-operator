package gtid

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/filemanager"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/gtid"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
)

type GtidHandler struct {
	fileManager    *filemanager.FileManager
	responseWriter *mdbhttp.ResponseWriter
	logger         *logr.Logger
}

func NewGtidHandler(fileManager *filemanager.FileManager, responseWriter *mdbhttp.ResponseWriter,
	logger *logr.Logger) router.RouteHandler {
	return &GtidHandler{
		fileManager:    fileManager,
		responseWriter: responseWriter,
		logger:         logger,
	}
}

func (h *GtidHandler) SetupRoutes(router *chi.Mux) {
	router.Route("/gtid", func(r chi.Router) {
		r.Get("/", h.GetGtid)
	})
}

type GtidResponse struct {
	Gtid string `json:"gtid"`
}

func (h *GtidHandler) GetGtid(w http.ResponseWriter, r *http.Request) {
	h.logger.V(1).Info("getting GTID")

	bytes, err := h.fileManager.ReadStateFile(gtid.MariaDBOperatorFileName)
	if err != nil {
		h.responseWriter.WriteErrorf(w, "error reading GTID file '%s': %v", gtid.MariaDBOperatorFileName, err)
		return
	}
	gtid, err := gtid.ParseRawGtidInMetaFile(bytes)
	if err != nil {
		h.responseWriter.WriteErrorf(w, "error parsing GTID: %v", err)
		return
	}

	h.responseWriter.WriteOK(w, GtidResponse{
		Gtid: gtid,
	})
}
