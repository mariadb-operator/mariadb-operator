package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
)

type ResponseWriter struct {
	logger *logr.Logger
}

func NewResponseWriter(logger *logr.Logger) *ResponseWriter {
	return &ResponseWriter{
		logger: logger,
	}
}

func (r *ResponseWriter) Write(w http.ResponseWriter, v any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		r.logger.Error(err, "error encoding json")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (r *ResponseWriter) WriteOK(w http.ResponseWriter, v any) {
	r.Write(w, v, http.StatusOK)
}

func (r *ResponseWriter) WriteError(w http.ResponseWriter, msg string) {
	r.Write(w, errors.NewAPIError(msg), http.StatusInternalServerError)
}

func (r *ResponseWriter) WriteErrorf(w http.ResponseWriter, format string, a ...any) {
	r.Write(w, errors.NewAPIErrorf(format, a...), http.StatusInternalServerError)
}
