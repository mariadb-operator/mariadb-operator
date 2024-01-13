package client

import (
	"context"
	"encoding/json"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type ServiceParameters struct {
	User     string    `json:"user"`
	Password string    `json:"password"`
	Params   MapParams `json:"-"`
}

func (s ServiceParameters) MarshalJSON() ([]byte, error) {
	type ServiceParametersInternal ServiceParameters // prevent recursion
	bytes, err := json.Marshal(ServiceParametersInternal(s))
	if err != nil {
		return nil, err
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(bytes, &rawMap); err != nil {
		return nil, err
	}

	for k, v := range s.Params {
		if _, ok := rawMap[k]; ok { // prevent overriding
			continue
		}
		bytes, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		rawMap[k] = bytes
	}

	return json.Marshal(rawMap)
}

type ServiceAttributes struct {
	Router     mariadbv1alpha1.ServiceRouter `json:"router"`
	Parameters ServiceParameters             `json:"parameters"`
}

type ServiceClient struct {
	client *mdbhttp.Client
}

func (s *ServiceClient) List(ctx context.Context) ([]Data[ServiceAttributes], error) {
	var list List[ServiceAttributes]
	res, err := s.client.Get(ctx, "services", nil)
	if err != nil {
		return nil, fmt.Errorf("error getting services: %v", err)
	}
	if err := handleResponse(res, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}

func (s *ServiceClient) Create(ctx context.Context, name string, router mariadbv1alpha1.ServiceRouter, params ServiceParameters,
	relationships Relationships) error {
	object := &Object[ServiceAttributes]{
		Data: Data[ServiceAttributes]{
			ID:   name,
			Type: ObjectTypeServices,
			Attributes: ServiceAttributes{
				Router:     router,
				Parameters: params,
			},
			Relationships: &relationships,
		},
	}
	res, err := s.client.Post(ctx, "services", object, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
