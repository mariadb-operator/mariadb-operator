package client

import (
	"encoding/json"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
)

type FilterParameters struct {
	Params MapParams `json:"-"`
}

func (f FilterParameters) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Params)
}

type FilterAttributes struct {
	Module     string           `json:"module"`
	Parameters FilterParameters `json:"parameters"`
}

type FilterClient struct {
	GenericClient[*FilterAttributes]
}

func NewFilterClient(client *mdbhttp.Client) *FilterClient {
	return &FilterClient{
		GenericClient: NewGenericClient[*FilterAttributes](
			client,
			"filters",
			ObjectTypeFilters,
		),
	}
}
