package client

import (
	"context"
	"encoding/json"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type ListenerParameters struct {
	Port     int32     `json:"port"`
	Protocol string    `json:"protocol"`
	Params   MapParams `json:"-"`
}

func (l ListenerParameters) MarshalJSON() ([]byte, error) {
	type ListenerParametersInternal ListenerParameters // prevent recursion
	bytes, err := json.Marshal(ListenerParametersInternal(l))
	if err != nil {
		return nil, err
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(bytes, &rawMap); err != nil {
		return nil, err
	}

	for k, v := range l.Params {
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

type ListenerAttributes struct {
	Parameters ListenerParameters `json:"parameters"`
}

type ListenerClient struct {
	client *mdbhttp.Client
}

func (s *ListenerClient) Create(ctx context.Context, name string, params ListenerParameters, relationships Relationships) error {
	object := &Object[ListenerAttributes]{
		Data: Data[ListenerAttributes]{
			ID:   name,
			Type: ObjectTypeListeners,
			Attributes: ListenerAttributes{
				Parameters: params,
			},
			Relationships: &relationships,
		},
	}
	res, err := s.client.Post(ctx, "listeners", object, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
