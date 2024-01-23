package client

import (
	"encoding/json"

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
	State      string                        `json:"state,omitempty"`
	Parameters ServiceParameters             `json:"parameters"`
}

type ServiceClient struct {
	GenericClient[ServiceAttributes]
}

func NewServiceClient(client *mdbhttp.Client) *ServiceClient {
	return &ServiceClient{
		GenericClient: NewGenericClient[ServiceAttributes](
			client,
			"services",
			ObjectTypeServices,
		),
	}
}
