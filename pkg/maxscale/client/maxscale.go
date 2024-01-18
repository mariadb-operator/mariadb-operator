package client

import (
	"encoding/json"
	"time"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type MaxScaleParameters struct {
	ConfigSyncUser     string        `json:"config_sync_user"`
	ConfigSyncPassword string        `json:"config_sync_password"`
	ConfigSyncDB       string        `json:"config_sync_db"`
	ConfigSyncInterval time.Duration `json:"config_sync_interval"`
	ConfigSyncTimeout  time.Duration `json:"config_sync_timeout"`
	Params             MapParams     `json:"-"`
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

type MaxscaleAttributes struct {
	Parameters MonitorParameters `json:"parameters"`
}

type MaxScaleClient struct {
	GenericClient[MaxscaleAttributes]
}

func NewMaxScaleClient(client *mdbhttp.Client) *MaxScaleClient {
	return &MaxScaleClient{
		GenericClient: NewGenericClient[MaxscaleAttributes](
			client,
			"maxscale",
			ObjectTypeMaxScale,
		),
	}
}
