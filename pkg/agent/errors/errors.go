package errors

import "fmt"

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
