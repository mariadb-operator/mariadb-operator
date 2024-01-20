package client

import (
	"context"
	"fmt"

	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type GenericClient[T any] struct {
	client     *mdbhttp.Client
	path       string
	objectType ObjectType
}

type Options struct {
	query         map[string]string
	relationships *Relationships
}

type Option func(o *Options)

func WithQuery(q map[string]string) Option {
	return func(o *Options) {
		o.query = q
	}
}

func WithForceQuery() Option {
	return func(o *Options) {
		o.query = map[string]string{
			"force": "true",
		}
	}
}

func WithRelationships(rels *Relationships) Option {
	return func(o *Options) {
		o.relationships = rels
	}
}

func NewGenericClient[T any](client *mdbhttp.Client, path string, objectType ObjectType) GenericClient[T] {
	return GenericClient[T]{
		client:     client,
		path:       path,
		objectType: objectType,
	}
}

func (c *GenericClient[T]) List(ctx context.Context, options ...Option) ([]Data[T], error) {
	opts := c.processOptions(options...)
	res, err := c.client.Get(ctx, c.path, opts.query)
	if err != nil {
		return nil, err
	}
	var list List[T]
	if err := handleResponse(res, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}

func (c *GenericClient[T]) ListIndex(ctx context.Context, options ...Option) (ds.Index[Data[T]], error) {
	list, err := c.List(ctx, options...)
	if err != nil {
		return nil, err
	}
	return ds.NewIndex[Data[T]](list, func(d Data[T]) string {
		return d.ID
	}), nil
}

func (c *GenericClient[T]) AllExists(ctx context.Context, ids []string, options ...Option) (bool, error) {
	index, err := c.ListIndex(ctx, options...)
	if err != nil {
		return false, nil
	}
	return ds.AllExists[Data[T]](index, ids...), nil
}

func (c *GenericClient[T]) Get(ctx context.Context, name string, options ...Option) (*Data[T], error) {
	opts := c.processOptions(options...)
	res, err := c.client.Get(ctx, c.resourcePath(name), opts.query)
	if err != nil {
		return nil, err
	}
	var object Object[T]
	if err := handleResponse(res, &object); err != nil {
		return nil, err
	}
	return &object.Data, nil
}

func (c *GenericClient[T]) Create(ctx context.Context, name string, attributes T, options ...Option) error {
	opts := c.processOptions(options...)
	object := &Object[T]{
		Data: Data[T]{
			ID:            name,
			Type:          c.objectType,
			Attributes:    attributes,
			Relationships: opts.relationships,
		},
	}
	res, err := c.client.Post(ctx, c.path, object, opts.query)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (c *GenericClient[T]) Delete(ctx context.Context, name string, options ...Option) error {
	opts := c.processOptions(options...)
	res, err := c.client.Delete(ctx, c.resourcePath(name), nil, opts.query)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (c *GenericClient[T]) Patch(ctx context.Context, name string, attributes T, options ...Option) error {
	opts := c.processOptions(options...)
	object := &Object[T]{
		Data: Data[T]{
			ID:            name,
			Type:          c.objectType,
			Attributes:    attributes,
			Relationships: opts.relationships,
		},
	}
	res, err := c.client.Patch(ctx, c.resourcePath(name), object, opts.query)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (c *GenericClient[T]) Put(ctx context.Context, name string, options ...Option) error {
	opts := c.processOptions(options...)
	res, err := c.client.Put(ctx, c.resourcePath(name), nil, opts.query)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (c *GenericClient[T]) Stop(ctx context.Context, name string, options ...Option) error {
	opts := c.processOptions(options...)
	res, err := c.client.Put(ctx, c.stopPath(name), nil, opts.query)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (c *GenericClient[T]) Start(ctx context.Context, name string, options ...Option) error {
	opts := c.processOptions(options...)
	res, err := c.client.Put(ctx, c.startPath(name), nil, opts.query)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (c *GenericClient[T]) resourcePath(name string) string {
	return fmt.Sprintf("%s/%s", c.path, name)
}

func (c *GenericClient[T]) stopPath(name string) string {
	return fmt.Sprintf("%s/stop", c.resourcePath(name))
}

func (c *GenericClient[T]) startPath(name string) string {
	return fmt.Sprintf("%s/start", c.resourcePath(name))
}

func (c *GenericClient[T]) processOptions(options ...Option) Options {
	opts := Options{}
	for _, setOpt := range options {
		if setOpt != nil {
			setOpt(&opts)
		}
	}
	return opts
}
