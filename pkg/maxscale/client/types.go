package client

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-multierror"
)

type ObjectType string

const (
	ObjectTypeMaxScale  ObjectType = "maxscale"
	ObjectTypeUsers     ObjectType = "inet"
	ObjectTypeServers   ObjectType = "servers"
	ObjectTypeMonitors  ObjectType = "monitors"
	ObjectTypeServices  ObjectType = "services"
	ObjectTypeListeners ObjectType = "listeners"
)

type Relationships struct {
	Data []struct {
		ID   string     `json:"id"`
		Type ObjectType `json:"type"`
	} `json:"data"`
}

type RelationshipsByType struct {
	Servers   *Relationships `json:"servers,omitempty"`
	Monitors  *Relationships `json:"monitors,omitempty"`
	Services  *Relationships `json:"services,omitempty"`
	Listeners *Relationships `json:"listeners,omitempty"`
}

type PayloadData[T any] struct {
	ID            string         `json:"id"`
	Type          ObjectType     `json:"type"`
	Attributes    T              `json:"attributes"`
	Relationships *Relationships `json:"relationships,omitempty"`
}

type Payload[T any] struct {
	Data PayloadData[T] `json:"data"`
}

type APIErrorItem struct {
	Detail string `json:"detail"`
}

type APIError struct {
	Errors []APIErrorItem `json:"errors"`
}

func (e *APIError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Detail
	}
	var aggErr *multierror.Error
	for _, err := range e.Errors {
		aggErr = multierror.Append(aggErr, errors.New(err.Detail))
	}
	return aggErr.Error()
}

func NewAPIError(message string) error {
	return &APIError{
		Errors: []APIErrorItem{
			{
				Detail: message,
			},
		},
	}
}

func NewAPIErrorf(format string, args ...any) error {
	return NewAPIError(fmt.Sprintf(format, args...))
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
