package client

import (
	"context"
	"encoding/json"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MonitorParameters struct {
	User            string          `json:"user"`
	Password        string          `json:"password"`
	MonitorInterval metav1.Duration `json:"monitor_interval,omitempty"`
	Params          MapParams       `json:"-"`
}

func (m MonitorParameters) MarshalJSON() ([]byte, error) {
	type MonitorParametersInternal MonitorParameters // prevent recursion
	bytes, err := json.Marshal(MonitorParametersInternal(m))
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

type MonitorAttributes struct {
	Module     mariadbv1alpha1.MonitorModule `json:"module"`
	Parameters MonitorParameters             `json:"parameters"`
}

type MonitorClient struct {
	ReadClient[MonitorAttributes]
	client *mdbhttp.Client
}

func NewMonitorClient(client *mdbhttp.Client) *MonitorClient {
	return &MonitorClient{
		ReadClient: NewListClient[MonitorAttributes](client, "monitors"),
		client:     client,
	}
}

func (m *MonitorClient) Create(ctx context.Context, name string, module mariadbv1alpha1.MonitorModule, params MonitorParameters,
	relationships Relationships) error {
	object := &Object[MonitorAttributes]{
		Data: Data[MonitorAttributes]{
			ID:   name,
			Type: ObjectTypeMonitors,
			Attributes: MonitorAttributes{
				Module:     module,
				Parameters: params,
			},
			Relationships: &relationships,
		},
	}
	res, err := m.client.Post(ctx, "monitors", object, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
