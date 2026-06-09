package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
)

var _ = Describe("EnvironmentHandler", func() {
	DescribeTable("SetValue",
		func(requestBody interface{}, expectedStatusCode int, expectedResponse map[string]string,
			expectFieldUpdate bool, updatedValue, initialPassword string) {
			podEnv := &environment.PodEnvironment{
				MariadbRootPassword: initialPassword,
			}
			logger := logr.Discard()
			responseWriter := mdbhttp.NewResponseWriter(&logger)
			handler := NewEnvironmentHandler(podEnv, responseWriter, &logger).(*EnvironmentHandler)

			bodyBytes, _ := json.Marshal(requestBody)
			req := httptest.NewRequestWithContext(context.Background(), "PUT", "/environment", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.SetValue(rr, req)

			Expect(rr.Code).To(Equal(expectedStatusCode))

			var responseBody map[string]string
			err := json.Unmarshal(rr.Body.Bytes(), &responseBody)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseBody).To(Equal(expectedResponse))

			if expectFieldUpdate {
				Expect(podEnv.MariadbRootPassword).To(Equal(updatedValue))
			} else {
				Expect(podEnv.MariadbRootPassword).To(Equal(initialPassword))
			}
		},
		Entry("Successful Update",
			map[string]string{
				"key":   "MARIADB_ROOT_PASSWORD",
				"value": "new-password",
			},
			http.StatusOK,
			map[string]string{
				"message": "'MARIADB_ROOT_PASSWORD' set successfully",
			},
			true,
			"new-password",
			"initial-password",
		),
		Entry("Key Not Found",
			map[string]string{
				"key":   "NON_EXISTENT_KEY",
				"value": "some-value",
			},
			http.StatusNotFound,
			map[string]string{
				"message": "key 'NON_EXISTENT_KEY' not found",
			},
			false,
			"",
			"initial-password",
		),
		Entry("Invalid Request Body - Malformed JSON",
			"invalid-json",
			http.StatusBadRequest,
			map[string]string{
				"message": "json: cannot unmarshal string into Go value of type handler.setValueRequest",
			},
			false,
			"",
			"initial-password",
		),
	)
})
