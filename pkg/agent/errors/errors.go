package errors

import (
	"fmt"
	"net/http"
)

type APIError struct {
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

func NewAPIError(message string) error {
	return &APIError{
		Message: message,
	}
}

func NewAPIErrorf(format string, a ...any) error {
	return &APIError{
		Message: fmt.Sprintf(format, a...),
	}
}

type Error struct {
	HTTPCode int
	Message  string
}

func (e *Error) Error() string {
	return e.Message
}

func NewError(httpCode int, message string) error {
	return &Error{
		HTTPCode: httpCode,
		Message:  message,
	}
}

func IsNotFound(err error) bool {
	if clientErr, ok := err.(*Error); ok {
		return clientErr.HTTPCode == http.StatusNotFound
	}
	return false
}
