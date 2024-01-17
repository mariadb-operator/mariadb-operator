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

func NewGenericClient[T any](client *mdbhttp.Client, path string, objectType ObjectType) GenericClient[T] {
	return GenericClient[T]{
		client:     client,
		path:       path,
		objectType: objectType,
	}
}

func (c *GenericClient[T]) List(ctx context.Context) ([]Data[T], error) {
	var list List[T]
	res, err := c.client.Get(ctx, c.path, nil)
	if err != nil {
		return nil, err
	}
	if err := handleResponse(res, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}

func (c *GenericClient[T]) ListIndex(ctx context.Context) (ds.Index[Data[T]], error) {
	list, err := c.List(ctx)
	if err != nil {
		return nil, err
	}
	return ds.NewIndex[Data[T]](list, func(d Data[T]) string {
		return d.ID
	}), nil
}

func (c *GenericClient[T]) AnyExists(ctx context.Context, ids ...string) (bool, error) {
	index, err := c.ListIndex(ctx)
	if err != nil {
		return false, nil
	}
	return ds.AnyExists[Data[T]](index, ids...), nil
}

func (c *GenericClient[T]) Get(ctx context.Context, name string) (*Data[T], error) {
	res, err := c.client.Get(ctx, c.resourcePath(name), nil)
	if err != nil {
		return nil, err
	}
	var object Object[T]
	if err := handleResponse(res, &object); err != nil {
		return nil, err
	}
	return &object.Data, nil
}

func (c *GenericClient[T]) Create(ctx context.Context, name string, attributes T, relationships *Relationships) error {
	object := &Object[T]{
		Data: Data[T]{
			ID:            name,
			Type:          c.objectType,
			Attributes:    attributes,
			Relationships: relationships,
		},
	}
	res, err := c.client.Post(ctx, c.path, object, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (c *GenericClient[T]) Delete(ctx context.Context, name string) error {
	res, err := c.client.Delete(ctx, c.resourcePath(name), nil, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (c *GenericClient[T]) resourcePath(name string) string {
	return fmt.Sprintf("%s/%s", c.path, name)
}
