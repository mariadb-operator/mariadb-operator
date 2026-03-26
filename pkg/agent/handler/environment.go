package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/errors"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
)

type EnvironmentHandler struct {
	responseWriter *mdbhttp.ResponseWriter
	logger         *logr.Logger
	environment    *environment.PodEnvironment
}

func NewEnvironmentHandler(environment *environment.PodEnvironment, responseWriter *mdbhttp.ResponseWriter,
	logger *logr.Logger) router.RouteHandler {
	return &EnvironmentHandler{
		responseWriter: responseWriter,
		logger:         logger,
		environment:    environment,
	}
}

func (e *EnvironmentHandler) SetupRoutes(router *chi.Mux) {
	router.Route("/environment", func(r chi.Router) {
		r.Put("/", e.SetValue)
	})
}

type setValueRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

// SetValue will overwrite a value given a key in the PodEnvironment. Matches based on the `env` struct tag
// {"key":"MARIADB_ROOT_PASSWORD","value":"your-new-password"}
func (e *EnvironmentHandler) SetValue(w http.ResponseWriter, r *http.Request) {
	var req setValueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		e.responseWriter.Write(w, http.StatusBadRequest, errors.NewAPIError(err.Error()))
		return
	}

	e.logger.Info("Setting a value", "key", req.Key)

	v := reflect.ValueOf(e.environment).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("env") // this is the struct tag
		envKey := strings.Split(tag, ",")[0]

		if envKey == req.Key {
			fieldValue := v.Field(i)
			if fieldValue.CanSet() {
				if fieldValue.Kind() == reflect.String {
					fieldValue.SetString(req.Value)
					e.responseWriter.Write(w, http.StatusOK, &MessageResponse{
						Message: fmt.Sprintf("'%s' set successfully", req.Key),
					})
					return
				}
			}
		}
	}

	e.responseWriter.Write(w, http.StatusNotFound, errors.NewAPIErrorf("key '%s' not found", req.Key))
}
