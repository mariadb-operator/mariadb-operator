package client

import (
	"context"
	"encoding/json"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MaxScaleParameters struct {
	ConfigSyncCluster  string          `json:"config_sync_cluster"`
	ConfigSyncUser     string          `json:"config_sync_user"`
	ConfigSyncPassword string          `json:"config_sync_password"`
	ConfigSyncDB       string          `json:"config_sync_db"`
	ConfigSyncInterval metav1.Duration `json:"config_sync_interval"`
	ConfigSyncTimeout  metav1.Duration `json:"config_sync_timeout"`
	Params             MapParams       `json:"-"`
}

func (m MaxScaleParameters) MarshalJSON() ([]byte, error) {
	type MaxScaleParametersInternal MaxScaleParameters // prevent recursion
	bytes, err := json.Marshal(MaxScaleParametersInternal(m))
	if err != nil {
		return nil, err
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(bytes, &rawMap); err != nil {
		return nil, err
	}

	for k, v := range m.Params {
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

type MaxScaleConfigSync struct {
	Version int `json:"version"`
}

type MaxScaleAttributes struct {
	ConfigSync *MaxScaleConfigSync `json:"config_sync,omitempty"`
	Parameters MaxScaleParameters  `json:"parameters"`
}

type MaxScaleClient struct {
	GenericClient[MaxScaleAttributes]
	client *mdbhttp.Client
}

func NewMaxScaleClient(client *mdbhttp.Client) *MaxScaleClient {
	return &MaxScaleClient{
		client: client,
	}
}

func (m *MaxScaleClient) Get(ctx context.Context) (*Data[MaxScaleAttributes], error) {
	res, err := m.client.Get(ctx, "maxscale", nil)
	if err != nil {
		return nil, err
	}
	var object Object[MaxScaleAttributes]
	if err := handleResponse(res, &object); err != nil {
		return nil, err
	}
	return &object.Data, nil
}

func (m *MaxScaleClient) Patch(ctx context.Context, attributes MaxScaleAttributes) error {
	object := &Object[MaxScaleAttributes]{
		Data: Data[MaxScaleAttributes]{
			Type:       ObjectTypeMaxScale,
			Attributes: attributes,
		},
	}
	res, err := m.client.Patch(ctx, "maxscale", object, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
