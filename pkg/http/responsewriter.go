package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
	mdbreflect "github.com/mariadb-operator/mariadb-operator/pkg/reflect"
)

type ResponseWriter struct {
	logger *logr.Logger
}

func NewResponseWriter(logger *logr.Logger) *ResponseWriter {
	return &ResponseWriter{
		logger: logger,
	}
}

func (r *ResponseWriter) Write(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if !mdbreflect.IsNil(v) {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			r.logger.Error(err, "error encoding json")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

func (r *ResponseWriter) WriteOK(w http.ResponseWriter, v any) {
	r.Write(w, http.StatusOK, v)
}

func (r *ResponseWriter) WriteError(w http.ResponseWriter, msg string) {
	r.Write(w, http.StatusInternalServerError, errors.NewAPIError(msg))
}

func (r *ResponseWriter) WriteErrorf(w http.ResponseWriter, format string, a ...any) {
	r.Write(w, http.StatusInternalServerError, errors.NewAPIErrorf(format, a...))
}
