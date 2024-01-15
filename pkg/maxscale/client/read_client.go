package client

import (
	"context"
	"fmt"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type ReadClient[T any] struct {
	client *mdbhttp.Client
	path   string
}

func NewListClient[T any](client *mdbhttp.Client, path string) ReadClient[T] {
	return ReadClient[T]{
		client: client,
		path:   path,
	}
}

func (c *ReadClient[T]) List(ctx context.Context) ([]Data[T], error) {
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

func (c *ReadClient[T]) ListIndex(ctx context.Context) (Index[T], error) {
	list, err := c.List(ctx)
	if err != nil {
		return nil, err
	}
	return IndexList(list), nil
}

func (c *ReadClient[T]) AnyExists(ctx context.Context, ids ...string) (bool, error) {
	index, err := c.ListIndex(ctx)
	if err != nil {
		return false, nil
	}
	return AnyExists(index, ids...), nil
}

func (c *ReadClient[T]) Get(ctx context.Context, name string) (*Data[T], error) {
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

func (c *ReadClient[T]) resourcePath(name string) string {
	return fmt.Sprintf("%s/%s", c.path, name)
}
