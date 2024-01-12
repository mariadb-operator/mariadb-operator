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

type RelationshipItem struct {
	ID   string     `json:"id"`
	Type ObjectType `json:"type"`
}

type RelationshipData struct {
	Data []RelationshipItem `json:"data"`
}

type Relationships struct {
	Servers   RelationshipData `json:"servers,omitempty"`
	Monitors  RelationshipData `json:"monitors,omitempty"`
	Services  RelationshipData `json:"services,omitempty"`
	Listeners RelationshipData `json:"listeners,omitempty"`
}

func ServerRelationships(servers ...string) Relationships {
	items := make([]RelationshipItem, len(servers))
	for i, srv := range servers {
		items[i] = RelationshipItem{
			ID:   srv,
			Type: ObjectTypeServers,
		}
	}
	return Relationships{
		Servers: RelationshipData{
			Data: items,
		},
	}
}

type Data[T any] struct {
	ID            string         `json:"id"`
	Type          ObjectType     `json:"type"`
	Attributes    T              `json:"attributes"`
	Relationships *Relationships `json:"relationships,omitempty"`
}

type Object[T any] struct {
	Data Data[T] `json:"data"`
}

type List[T any] struct {
	Data []Data[T] `json:"data"`
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

func IsUnautorized(err error) bool {
	return HasStatusCode(err, http.StatusUnauthorized)
}

func IsNotFound(err error) bool {
	return HasStatusCode(err, http.StatusNotFound)
}

func HasStatusCode(err error, statusCode int) bool {
	if clientErr, ok := err.(*Error); ok {
		return clientErr.HTTPCode == statusCode
	}
	return false
}
