package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/hashicorp/go-multierror"
)

type ObjectType string

const (
	ObjectTypeUsers     ObjectType = "inet"
	ObjectTypeServers   ObjectType = "servers"
	ObjectTypeMonitors  ObjectType = "monitors"
	ObjectTypeServices  ObjectType = "services"
	ObjectTypeListeners ObjectType = "listeners"
	ObjectTypeMaxScale  ObjectType = "maxscale"
)

type RelationshipItem struct {
	ID   string     `json:"id"`
	Type ObjectType `json:"type"`
}

type RelationshipData struct {
	Data []RelationshipItem `json:"data,omitempty"`
}

type Relationships struct {
	Servers   *RelationshipData `json:"servers,omitempty"`
	Monitors  *RelationshipData `json:"monitors,omitempty"`
	Services  *RelationshipData `json:"services,omitempty"`
	Listeners *RelationshipData `json:"listeners,omitempty"`
}

type RelationshipsBuilder struct {
	rels *Relationships
}

func NewRelationshipsBuilder() *RelationshipsBuilder {
	return &RelationshipsBuilder{
		rels: &Relationships{},
	}
}

func (b *RelationshipsBuilder) WithServers(servers ...string) *RelationshipsBuilder {
	b.rels.Servers = &RelationshipData{
		Data: b.items(ObjectTypeServers, servers...),
	}
	return b
}

func (b *RelationshipsBuilder) WithMonitors(monitors ...string) *RelationshipsBuilder {
	b.rels.Monitors = &RelationshipData{
		Data: b.items(ObjectTypeMonitors, monitors...),
	}
	return b
}

func (b *RelationshipsBuilder) WithServices(services ...string) *RelationshipsBuilder {
	b.rels.Services = &RelationshipData{
		Data: b.items(ObjectTypeServices, services...),
	}
	return b
}

func (b *RelationshipsBuilder) WithListeners(listeners ...string) *RelationshipsBuilder {
	b.rels.Listeners = &RelationshipData{
		Data: b.items(ObjectTypeListeners, listeners...),
	}
	return b
}

func (b *RelationshipsBuilder) Build() *Relationships {
	return b.rels
}

func (b *RelationshipsBuilder) items(objType ObjectType, ids ...string) []RelationshipItem {
	items := make([]RelationshipItem, len(ids))
	for i, id := range ids {
		items[i] = RelationshipItem{
			ID:   id,
			Type: objType,
		}
	}
	return items
}

type Param string

// See: https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#special-parameter-types
func (p Param) MarshalJSON() ([]byte, error) {
	if i, err := strconv.ParseInt(string(p), 10, 32); err == nil {
		return json.Marshal(i)
	}
	if b, err := strconv.ParseBool(string(p)); err == nil {
		return json.Marshal(b)
	}
	// Supported by MaxScale and not by strconv.ParseBool
	if p == "yes" || p == "on" {
		return json.Marshal(true)
	}
	// Supported by MaxScale and not by strconv.ParseBool
	if p == "no" || p == "off" {
		return json.Marshal(false)
	}
	type ParamInternal Param // prevent recursion
	return json.Marshal(ParamInternal(p))
}

type MapParams map[string]Param

func NewMapParams(params map[string]string) map[string]Param {
	mapParams := make(map[string]Param, len(params))
	for k, v := range params {
		mapParams[k] = Param(v)
	}
	return mapParams
}

type Data[T any] struct {
	ID            string         `json:"id,omitempty"`
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
