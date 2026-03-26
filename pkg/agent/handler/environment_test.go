package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
	"github.com/stretchr/testify/assert"
)

func TestSetValue(t *testing.T) {
	testCases := []struct {
		name               string
		requestBody        interface{}
		expectedStatusCode int
		expectedResponse   map[string]string
		expectFieldUpdate  bool
		updatedValue       string
		initialPassword    string
	}{
		{
			name: "Successful Update",
			requestBody: map[string]string{
				"key":   "MARIADB_ROOT_PASSWORD",
				"value": "new-password",
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse: map[string]string{
				"message": "'MARIADB_ROOT_PASSWORD' set successfully",
			},
			expectFieldUpdate: true,
			updatedValue:      "new-password",
			initialPassword:   "initial-password",
		},
		{
			name: "Key Not Found",
			requestBody: map[string]string{
				"key":   "NON_EXISTENT_KEY",
				"value": "some-value",
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse: map[string]string{
				"message": "key 'NON_EXISTENT_KEY' not found",
			},
			expectFieldUpdate: false,
			initialPassword:   "initial-password",
		},
		{
			name:               "Invalid Request Body - Malformed JSON",
			requestBody:        "invalid-json",
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse: map[string]string{
				"message": "json: cannot unmarshal string into Go value of type handler.setValueRequest",
			},
			expectFieldUpdate: false,
			initialPassword:   "initial-password",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			podEnv := &environment.PodEnvironment{
				MariadbRootPassword: tc.initialPassword,
			}
			logger := logr.Discard()
			responseWriter := mdbhttp.NewResponseWriter(&logger)
			handler := NewEnvironmentHandler(podEnv, responseWriter, &logger).(*EnvironmentHandler)

			bodyBytes, _ := json.Marshal(tc.requestBody)
			req := httptest.NewRequestWithContext(context.Background(), "PUT", "/environment", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.SetValue(rr, req)

			assert.Equal(t, tc.expectedStatusCode, rr.Code, "status code should match")

			var responseBody map[string]string
			err := json.Unmarshal(rr.Body.Bytes(), &responseBody)
			assert.NoError(t, err, "response body should be valid json")
			assert.Equal(t, tc.expectedResponse, responseBody, "response body should match")

			if tc.expectFieldUpdate {
				assert.Equal(t, tc.updatedValue, podEnv.MariadbRootPassword, "field should be updated")
			} else {
				assert.Equal(t, tc.initialPassword, podEnv.MariadbRootPassword, "field should not be updated")
			}
		})
	}
}
